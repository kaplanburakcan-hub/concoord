# ADR-0007 — Faz 6: Günlük/Haftalık Saha Raporlama Kararları

Tarih: 07.07.2026 · Durum: Kabul edildi

## 1. Geçmiş gün düzeltmesi = revizyon (yerinde düzenleme değil)

**Karar:** `daily_reports` tablosuna `revision_no` + `parent_report_id`
eklendi; `UNIQUE (project_id, report_date, revision_no)`. `Submitted` rapor
ve satırları, hakedişteki `Finalized` kilidiyle aynı desenle **DB trigger**
seviyesinde değişmez (SQLSTATE 23001 → API 409). Düzeltme `/revise` ucuyla
tüm veriyi kopyalayan yeni `Draft` revizyon açar; geçiş
`workflow_transitions`a yazılır.

**Gerekçe:** Plan §5.1 "hiçbir değişiklik izsiz kalmaz" ilkesinin doğrudan
uygulaması. Günlük rapor ileride hakediş metrajının ve İSG kayıtlarının kanıt
zinciridir; yerinde düzenleme bu zinciri kırar. Uygulama katmanı kontrolü
yeterli görülmedi — kilit veritabanında, geliştirici unutamaz.

## 2. Haftalık PDF: snapshot API'de, çizim worker'da

**Karar:** `POST /weekly-reports` haftanın verisini **senkron** derleyip
`weekly_reports.snapshot` JSONB'ye dondurur; PDF üretimi `weekly_report_pdf`
işi olarak mevcut PostgreSQL kuyruğuna (Faz 4, river deseni) atılır. Worker
PDF'i YALNIZCA snapshot'tan çizer, MinIO'ya yükler, `files` kaydı açar,
statüyü `Ready` yapar ve üreteni bilgilendirir.

**Gerekçe:** Kabul kriteri "PDF rakamları snapshot'tan doğrulanabiliyor" —
veri dondurma anı, kullanıcının butona bastığı andır; kuyruk gecikmesi ya da
eşzamanlı revizyonlar sonucu etkilemez. Snapshot `GET` ucuyla ham döner;
birim testi (`TestWeeklyPDFFromSnapshot`) PDF'teki toplamların snapshot
değerleriyle birebir aynı olduğunu doğrular.

## 3. PDF motoru: Faz 3 stdlib yaklaşımı sürdürüldü

**Karar:** gotenberg/chromedp bu fazda da devreye alınmadı; haftalık PDF,
payments/pdf.go ile aynı minimal stdlib motoruyla (Base-14 Helvetica,
ASCII'ye harf çevrimi) üretilir. Motor `reports` paketinde yinelendi
(paketler arası bağımlılık yerine ~150 satır bilinçli kopya).

**Gerekçe:** ADR-0004 ile tutarlılık: sıfır yeni bağımlılık, deterministik
çıktı, VPS'te ek container yok. Zengin HTML→PDF şablonu Faz 9 aylık yönetim
raporlarıyla birlikte tek seferde değerlendirilecek; iki fazda iki ayrı
motor kurulumundan kaçınıldı.

## 4. Haftalık raporda finansal tutar yok

**Karar:** Snapshot bekleyen hakedişlerin yalnızca **statülerini** taşır
(taşeron, dönem no, durum); tutar/birim fiyat taşımaz.

**Gerekçe:** Plan §4 `view ≠ view_financials` ayrımı. Haftalık rapor
`reports.view` ile indirilebilir (saha ekibi dahil); tutar eklemek maskeleme
katmanı gerektirirdi. Finansal içerik `view_financial_reports` korumalı
aylık EVM raporunun (Faz 9) işidir.

## 5. PWA çevrimdışı kuyruk: localStorage + sıralı oynatma (Background Sync değil)

**Karar:** Çevrimdışı yazma kuyruğu uygulama katmanında (`offline/queue.ts`,
localStorage) tutulur; `online` olayı + açılış + periyodik tetikleyiciyle
**sırayla** oynatılır (create → update → submit bağımlılığı korunur;
sunucu id'si `{local:…}` yer tutucusuyla çözülür). Service worker yalnızca
uygulama kabuğunu cache'ler; API istekleri hiçbir zaman cache'lenmez.
409/4xx alan öğe `conflict/failed` işaretlenir ve kullanıcı kararına
bırakılır.

**Gerekçe:** Background Sync API iOS Safari'de yok; saha cihazlarının önemli
bölümü iOS. localStorage v1 için yeterli (yalnızca JSON gövde; fotoğraflı
çevrimdışı form Faz 8'de IndexedDB ile gelecek). Sessiz veri kaybı ve sessiz
üzerine yazma iki kırmızı çizgidir: kuyruk ya teslim eder ya kullanıcıya sorar.

## 6. Hava durumu ön doldurma: opsiyonel, backend proxy, Open-Meteo varsayılan

**Karar:** `IPKS_WEATHER_ENABLED` kapalıysa uç 404 döner ve hiçbir dış çağrı
yapılmaz. Açıksa backend, istemciden gelen koordinatla (cihaz GPS)
Open-Meteo'yu proxy'ler; sağlayıcı `IPKS_WEATHER_API_URL` ile değiştirilir.
Değer her zaman ÖN DOLDURMADIR — kullanıcı düzenleyebilir, kaynak
`weather.source` alanında saklanır.

**Gerekçe:** Plan bu özelliği açıkça opsiyonel işaretler. Proxy: kısıtlı saha
ağlarında dış API'ye doğrudan çıkış garantisi yok; sağlayıcı değişimi tek
dosyada (Plan §2 adaptör ilkesi). Open-Meteo anahtarsızdır — sır yönetimi
gerektirmez. `projects.location` serbest metin olduğundan koordinat
sunucuda türetilmez; konum cihazdan gelir.
