# İPKS — Prod Yayına Alma Kontrol Listesi (Faz 10 Kabul)

Plan §8 Faz 10 kabul kriteri: **bu liste eksiksiz** ve **DR tatbikatı belgelenmiş
şekilde geçildi**. Her madde işaretlenip tarih/sorumlu yazılmalıdır.

## A. Altyapı ve dağıtım
- [ ] VPS hazır (`vps-setup.sh`), güvenlik duvarı yalnızca 80/443 açık
- [ ] `docker compose ... prod.yml up -d` sağlıklı; tüm servisler `restart: unless-stopped`
- [ ] Postgres/MinIO host portları prod'da **kapalı** (yalnızca iç ağ)
- [ ] `make migrate-up` uygulanmış (000011 dahil)
- [ ] `make seed` çalıştırılmış; bootstrap admin ile giriş doğrulandı

## B. Güvenlik
- [ ] `IPKS_JWT_SECRET` güçlü ve gizli (dev varsayılanı DEĞİL)
- [ ] `IPKS_ENV=production` (JWT secret zorunluluğu aktif)
- [ ] HTTPS canlı; HTTP→HTTPS yönlendirme çalışıyor; sertifika otomatik yenileme kurulu
- [ ] Güvenlik başlıkları yanıtta mevcut (nosniff, X-Frame-Options, Referrer-Policy)
- [ ] Rate limit aktif ve makul (`RATE_LIMIT_RPS`, `LOGIN_RATE_LIMIT`)
- [ ] CORS allowlist doğru (aynı-origin ise boş)
- [ ] Dosya yükleme doğrulaması test edildi (geçerli dosya kabul, sahte-tip ret)
- [ ] (Opsiyonel) ClamAV kurulu ve `IPKS_CLAMD_ADDR` set; fail-closed doğrulandı
- [ ] Otomatik yetki test matrisi yeşil (her rol × kritik uç) — `make test`

## C. Performans
- [ ] `make loadtest` 100 eşzamanlı kullanıcıda eşikleri geçti (p95<800ms, hata<%1)
- [ ] Sıcak sorgular indeks kullanıyor (`EXPLAIN` ile teyit; 000011 sonrası)
- [ ] N+1 taraması temiz (`docs/faz10-performans-nplus1.md`)

## D. Dayanıklılık ve yedekleme
- [ ] Günlük offsite yedek cron'u kurulu (`crontab.example`)
- [ ] WAL arşivleme aktif
- [ ] Haftalık VPS snapshot ayarlı (sağlayıcı)
- [ ] `make restore-test` geçiyor
- [ ] **`make dr-drill` (PG + MinIO) geçti ve `docs/runbook-dr.md`'ye kaydedildi**
- [ ] Başarısız yedek uyarısı test edildi (webhook/log)

## E. İzleme
- [ ] `/healthz` ve `/readyz` yeşil; nginx healthcheck çalışıyor
- [ ] `IPKS_ERROR_WEBHOOK_URL` set ve 5xx bildirimi test edildi
- [ ] Log toplama/rotasyon planı belli (`make logs` / docker log driver)

## F. PWA / istemci
- [ ] Service worker prod build'de kaydoluyor; çevrimdışı sayfa (`/offline.html`) görünüyor
- [ ] Çevrimdışı kuyruk: uçak modunda girilen rapor bağlantıda senkronize oluyor
- [ ] Ana ekrana ekleme (manifest) mobilde çalışıyor

## G. İşlevsel duman testi (kritik uçtan uca)
- [ ] Login → proje seç → dashboard (EVM değerleri makul)
- [ ] Hakediş uçtan uca (taslak → onay → kesinleşme → kilit)
- [ ] Doküman v1→v2 versiyonlama + indirme; yetkisiz erişim reddi
- [ ] MAR karar + bildirim
- [ ] Günlük rapor → haftalık PDF
- [ ] PR → PO → teslimat
- [ ] İSG ceza → PDF + sonraki hakedişte kesinti önerisi
- [ ] Aylık yönetim raporu PDF tek tıkla üretiliyor

## H. Belgeler ve devir
- [ ] Kullanıcı kılavuzu (`docs/kilavuz-kullanici.md`) teslim edildi
- [ ] Admin kılavuzu (`docs/kilavuz-admin.md`) teslim edildi
- [ ] DR runbook (`docs/runbook-dr.md`) teslim edildi
- [ ] Devir teslim belgesi (`docs/devir-teslim.md`) imzalandı

---
**Onay:** _________________  **Tarih:** ___________  **Sürüm:** 1.0.0
