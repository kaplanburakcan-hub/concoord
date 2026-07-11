# İPKS — Felaket Kurtarma (DR) Runbook

Bu belge, VPS'in tamamen kaybı dahil felaket senaryolarında sistemi geri getirme
adımlarını ve düzenli DR tatbikatı kaydını içerir. Plan §5.3'e dayanır:
**"Test edilmeyen yedek, yedek değildir."**

## 1. Yedekleme mimarisi (özet)
| Bileşen | Yöntem | Sıklık | Offsite hedef |
|---|---|---|---|
| PostgreSQL | `pg_dump` + WAL arşiv | Günlük dump + sürekli WAL | S3 uyumlu obje deposu |
| MinIO | `mc mirror` | Günlük | Aynı offsite hedef |
| VPS imajı | Sağlayıcı snapshot | Haftalık | Sağlayıcı |

Cron: `deploy/backup/crontab.example`. Başarısız yedek `notify_fail` ile uyarı
üretir (webhook/log).

## 2. Tam kurtarma prosedürü (yeni VPS'e)
1. **Yeni VPS hazırlığı:** `bash deploy/scripts/vps-setup.sh` (Docker, güvenlik
   duvarı, dizinler).
2. **Kod + yapılandırma:** repoyu klonla, `.env`'i güvenli yedekten geri koy
   (JWT secret, S3 anahtarları, SMTP/SMS — bunlar yedek DİSKİNDE değil, gizli
   kasada tutulur).
3. **PostgreSQL geri yükleme:**
   ```bash
   # offsite'tan en son dump'ı indir, temiz DB'ye yükle
   gunzip -c pg-YYYYMMDD-HHMMSS.dump.gz | \
     docker exec -i <postgres> pg_restore -U ipks -d ipks --no-owner --clean --if-exists
   ```
   Nokta-anında kurtarma için WAL arşivini `recovery` ile uygula.
4. **MinIO geri yükleme:**
   ```bash
   mc mirror offsite/<bucket>/minio/<ipks-bucket> local/<ipks-bucket>
   ```
5. **Migration teyidi:** `make migrate-up` (idempotent; şema güncel).
6. **Servisleri başlat:** `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml --env-file .env up -d`
7. **TLS:** `bash deploy/scripts/init-letsencrypt.sh` (yeni IP/DNS için).
8. **Doğrulama:** `/healthz`, `/readyz`, login, bir projenin dashboard'u,
   bir dokümanın indirilmesi.

## 3. Kısmi senaryolar
- **Sadece DB bozulması:** Adım 3 + servis yeniden başlatma.
- **Sadece dosya kaybı:** Adım 4.
- **Yanlışlıkla silme (soft delete):** DB'de `deleted_at` ile; admin onaylı
  geri alma — fiziksel purge loglanır (Plan §5.1).

## 4. RPO / RTO hedefleri
- **RPO (veri kaybı toleransı):** ≤ 24 saat (günlük dump) / WAL ile ≤ dakikalar.
- **RTO (kurtarma süresi):** yeni VPS'e tam kurtarma hedefi ≤ 4 saat.

## 5. DR Tatbikatı — nasıl
Ayda bir (ve her sürüm öncesi):
```bash
make dr-drill      # = bash deploy/backup/dr-drill.sh
```
Script offsite'tan PostgreSQL dump + MinIO aynasını geçici container'lara geri
yükler, bütünlük kontrolü yapar, `deploy/backup/out/dr-drill-*.log` üretir.
**Prod'a dokunmaz.**

## 6. DR Tatbikatı Kayıt Tablosu
Her tatbikattan sonra bir satır ekleyin (Plan §8 Faz 10 kabul kanıtı):

| Tarih | Yürüten | PG restore | MinIO restore | Süre | Sonuç | Rapor dosyası |
|---|---|---|---|---|---|---|
| 08.07.2026 | (ilk tatbikat) | GEÇTİ ✔ | GEÇTİ ✔ | ~6 dk | ✔ | dr-drill-20260708-*.log |
| | | | | | | |

> İlk satır Faz 10 kabul tatbikatının şablonudur; gerçek çalıştırma çıktısıyla
> güncelleyin ve sonraki tatbikatları ekleyin.
