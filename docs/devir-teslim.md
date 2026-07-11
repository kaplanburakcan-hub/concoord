# İPKS — Devir Teslim Belgesi

**Ürün:** İnşaat Proje Koordinasyon ve Saha Takip Platformu (İPKS)
**Sürüm:** 1.0.0 (Faz 0–10 tamamlandı)
**Tarih:** 08.07.2026

## 1. Kapsam durumu
Program planındaki (v1.1) tüm fazlar tamamlandı:

| Faz | İçerik | Durum |
|---|---|---|
| 0 | Altyapı, Docker Compose, migration, yedekleme+restore testi, VPS deploy | ✔ |
| 1 | Kimlik/JWT + RBAC motoru + Admin Paneli v1 | ✔ |
| 2 | Proje çekirdeği + çoklu proje + doküman motoru (versiyonlama) | ✔ |
| 3 | Taşeron + hakediş finansal çekirdeği (kümülatif, kilitleme) | ✔ |
| 4 | Görev yönetimi (Kanban) + merkezi bildirim motoru | ✔ |
| 5 | Malzeme onayı (MAR / submittals) | ✔ |
| 6 | Günlük/haftalık saha raporlama + PWA offline v1 | ✔ |
| 7 | Satınalma ve tedarik zinciri (PR→PO→teslimat) | ✔ |
| 8 | İSG (checklist, denetim, bulgu, ceza otomasyonu) | ✔ |
| 9 | Dashboard, EVM, portföy, aylık yönetim raporu | ✔ |
| 10 | Sertleştirme ve yayına alma | ✔ |

## 2. Faz 10 teslim edilenler
**Güvenlik:** İstek hız sınırlama (genel + kimlik uçları), CORS allowlist, güvenlik
başlıkları, dosya tipi doğrulama (magic-byte + allowlist), opsiyonel ClamAV
taraması.
**Performans:** Ek performans indeksleri (migration 000011), index/N+1 analiz
belgesi, k6 yük testi (100 eşzamanlı kullanıcı) + duman testi.
**Dayanıklılık:** Tam DR tatbikat scripti (PostgreSQL + MinIO restore), DR runbook.
**İzleme:** healthz/readyz + 5xx/panik webhook bildirimi.
**PWA cilası:** Sürümlü service worker, çevrimdışı yedek sayfası, güncelleme akışı.
**Belgeler:** Kullanıcı kılavuzu, admin kılavuzu, prod go-live checklist, bu belge.

## 3. Mimari ilkeler (korunmalı)
1. **Hiçbir veri kaybolmaz, hiçbir değişiklik izsiz kalmaz** — soft delete, audit
   trail, doküman versiyonlama, iş akışı geçiş logları, finansal kilitleme.
2. **Yetki koda gömülmez, veriye yazılır** — RBAC matrisi + kullanıcı override
   (DENY > GRANT > rol).
3. **Her modül bir PMBOK alanına ve kontrol ritmine bağlı** — günlük/haftalık/aylık.

## 4. Teknoloji ve çalıştırma
- Backend: Go (chi + pgx), tek statik binary. Frontend: React/Vite/TS/Tailwind, PWA.
- Veritabanı: PostgreSQL 16. Depolama: MinIO (S3 uyumlu). Kuyruk: PostgreSQL (river deseni).
- Dağıtım: Docker Compose @ VPS + nginx + Let's Encrypt.
- Günlük komutlar: `make up|down|logs|migrate-up|seed|test|backup|restore-test|dr-drill|loadtest|smoke`.

## 5. Gizli bilgiler (ayrı, güvenli kanaldan)
`.env` içindeki sırlar bu repoda **yoktur** ve gizli kasada tutulur:
`IPKS_JWT_SECRET`, S3/MinIO anahtarları, SMTP/SMS kimlikleri, offsite S3
kimlikleri, `IPKS_ERROR_WEBHOOK_URL`. Devir sırasında bu değerler ayrı ve güvenli
şekilde aktarılır; yedek diskinde tutulmaz.

## 6. Bilinen sınırlar / Faz 11+ backlog (şema hazır)
Fiyat farkı (eskalasyon) · merkezi taşeron kartoteksi · Gantt/CPM · muhasebe
entegrasyonu (Logo/Mikro) · e-İmza · native mobil · çok dilli arayüz (EN).
Rate limit süreç-içidir; çok-replikalı yatay ölçeklemede Redis tabanlı uygulamaya
geçiş gerekir (arayüz hazır).

## 7. Destek ve bakım rutini
- **Günlük:** Yedek başarısı ve hata bildirimlerini izleyin.
- **Aylık:** `make restore-test` + `make dr-drill`; sonucu runbook'a işleyin.
- **Sürüm öncesi:** `make dr-drill` → `make migrate-up` → `make smoke`/`loadtest`.
- **Sertifika:** Otomatik yenilenir; certbot loglarını periyodik kontrol edin.

## 8. Kabul
Faz 10 kabul kriteri — prod checklist (`docs/prod-go-live-checklist.md`) eksiksiz
ve DR tatbikatı (`docs/runbook-dr.md`) belgelenmiş şekilde geçilmiştir.

**Teslim eden:** _______________  **Teslim alan:** _______________  **Tarih:** __________
