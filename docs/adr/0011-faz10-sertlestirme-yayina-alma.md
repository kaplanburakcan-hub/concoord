# ADR 0011 — Faz 10: Sertleştirme ve Yayına Alma

**Durum:** Kabul edildi
**Tarih:** 08.07.2026
**Bağlam fazı:** Faz 10 (Plan §8) — son faz, prod'a çıkış

## Bağlam
Faz 0–9 ile tüm iş modülleri tamamlandı. Yayına almadan önce güvenlik,
performans, dayanıklılık ve operasyonel hazırlık sertleştirilmeli; kabul kriteri
"prod checklist eksiksiz + belgelenmiş DR tatbikatı geçti".

## Kararlar

### 1. Hız sınırlama süreç-içi (in-memory), Redis'siz
Tek VPS / tek api replikası dağıtımı için token-bucket (genel) + sabit-pencere
(kimlik uçları) süreç-içi uygulandı. **Gerekçe:** Plan §2 minimalist yığını
korunur (Redis bağımlılığı yok, tıpkı river yerine PostgreSQL kuyruğunda olduğu
gibi). Yatay ölçekleme gerekirse aynı arayüz arkasına Redis tabanlı uygulama
geçilebilir.

### 2. CORS'ta joker yok
`*` origin bilinçli olarak desteklenmez; kimlik bilgili (Authorization)
isteklerde güvensizdir. Varsayılan aynı-origin (nginx tek host). **Gerekçe:**
En küçük saldırı yüzeyi; ihtiyaç halinde açık allowlist.

### 3. Yükleme doğrulaması: sihirli-bayt + allowlist, antivirüs opsiyonel
Tarayıcının `Content-Type`'ına güvenilmez; içerik koklanır ve uzantı allowlist'i
ile çapraz kontrol edilir. ClamAV **opsiyonel** (config ile) ve **fail-closed**.
**Gerekçe:** Depolamaya yürütülebilir/sahte-tip dosya girişini engellemek;
antivirüs kurulumunu her ortama zorlamadan kancayı hazır tutmak.

### 4. İzleme: log birincil, webhook opsiyonel; healthz/readyz ayrımı
Yapılandırılmış log her zaman birincil kayıttır. 5xx/panik olayları opsiyonel
Slack/Teams uyumlu webhook'a **asenkron, best-effort** gönderilir — izleme
kanalı istek yolunu bloklamaz. **Gerekçe:** Basit, bağımlılıksız, prod'da yeterli.

### 5. DR tatbikatı ayrı script, prod'a dokunmaz
`dr-drill.sh` offsite'tan PostgreSQL dump'ı + MinIO aynasını **geçici**
container'lara geri yükler, bütünlük kontrolü yapar, zaman damgalı rapor üretir.
`restore-test.sh` (sadece PG) korunur; DR tatbikatı onu MinIO ile tamamlar.
**Gerekçe:** Plan §5.3 "test edilmeyen yedek, yedek değildir" ilkesinin tam
(veri + dosya) tatbikatı.

### 6. Performans: indeks katmanı + N+1 doğrulaması
Şema zaten iyi indeksliydi; yalnızca gerçek boşluklar (EVM finalized yolu,
otomasyon köprüsü, versiyon join'i) eklendi. Redundant indeks (PK ile kapsanan)
eklenmedi. N+1 taraması kod yollarını temiz buldu.

## Sonuç
Sistem 1.0.0 sürümüyle prod'a hazır. Kabul: `docs/prod-go-live-checklist.md`
tamamlanır ve `dr-drill.sh` çıktısı `docs/runbook-dr.md` kayıt tablosuna işlenir.

## Alternatifler (reddedilen)
- **nginx düzeyi rate limit:** Uygulama düzeyi standart hata zarfı ve per-uç
  ayrım (kimlik uçları) sağladığı için tercih edildi; nginx katmanı ek olarak
  kullanılabilir.
- **WAF/harici antivirüs servisi:** v1 kapsamı için ağır; clamd kancası yeterli.
