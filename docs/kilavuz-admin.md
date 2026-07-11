# İPKS Yönetici (Admin) Kılavuzu

Platform yöneticileri ve operasyon sorumluları için kurulum, yetkilendirme ve
bakım rehberi.

## 1. İlk kurulum (VPS)
1. `bash deploy/scripts/vps-setup.sh` — Docker, güvenlik duvarı, dizinler.
2. `.env` dosyasını `.env.example`'dan üretip doldurun (aşağıdaki §7).
3. `bash deploy/scripts/init-letsencrypt.sh` — HTTPS sertifikası.
4. `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml --env-file .env up -d`
5. `make migrate-up` — şema.
6. `make seed` — izin sözlüğü + 7 rol + bootstrap admin (`IPKS_BOOTSTRAP_ADMIN_*`).
7. `/healthz` ve `/readyz` yeşil, HTTPS'ten giriş çalışıyor mu doğrulayın.

## 2. Kullanıcı yönetimi
Admin Paneli → Kullanıcılar: oluştur / düzenle / pasifleştir. Silme soft-delete'tir
(veri kaybolmaz, audit log tutar).

## 3. Roller ve izinler (RBAC)
- **7 varsayılan rol:** Admin, ProjectManager, SiteEngineer, SubcontractorRep,
  Client, ProcurementOfficer, OHSExpert.
- **İzin matrisi:** Kullanıcı × `modül.aksiyon` (checkbox grid), proje bazlı filtre.
- **Öncelik:** `DENY > GRANT > rol varsayılanı`. Bir kullanıcıya proje bazında tek
  bir izni açıp kapatabilirsiniz; etki anında API'de geçerlidir.
- **Finansal görünürlük:** `view` ≠ `view_financials`. Saha mühendisine metrajı
  gösterip tutar kolonlarını gizlemek için `view_financials`'ı vermeyin.
- **Yeni izin senaryosu kod gerektirmez** — matristen yönetilir.

## 4. Proje üyeliği
Admin Paneli → Proje Üyeleri: kullanıcıyı projeye rolüyle ekleyin. Taşeron
temsilcisini eklerken firmasını (`subcontractor`) bağlayın — satır seviyesi
güvenlik buna dayanır.

## 5. İSG checklist şablonları
Admin, denetim checklist şablonlarını (JSONB) tanımlar; saha bunları kullanır.
Yeni kontrol eklemek konfigürasyondur, kod değişikliği değil.

## 6. Denetim izi (audit log)
Admin Paneli → Denetim İzi: her INSERT/UPDATE/DELETE (aktör, varlık, öncesi/sonrası,
IP, zaman). İş akışı geçişleri ayrıca `workflow_transitions`ta.

## 7. Faz 10 güvenlik ayarları (.env)
| Değişken | Anlamı | Öneri |
|---|---|---|
| `IPKS_CORS_ORIGINS` | İzinli origin allowlist'i | Aynı-origin dağıtımda boş |
| `IPKS_RATE_LIMIT_RPS` / `_BURST` | Genel hız sınırı (IP/sn) | 20 / 40 |
| `IPKS_LOGIN_RATE_LIMIT` | Kimlik uçları dakikalık deneme | 10 |
| `IPKS_CLAMD_ADDR` | ClamAV clamd adresi (host:port) | Kuruluysa doldurun |
| `IPKS_ERROR_WEBHOOK_URL` | 5xx/panik bildirim webhook'u | Slack/Teams URL'i |
| `IPKS_JWT_SECRET` | Access token imza anahtarı | **Prod'da zorunlu, gizli** |

Değişiklik sonrası `docker compose ... up -d` ile api'yi yeniden başlatın.

### Antivirüs (opsiyonel)
`clamav/clamav` container'ını ekleyip `IPKS_CLAMD_ADDR=clamav:3310` verin.
Açıkken clamd erişilemezse yüklemeler **reddedilir** (fail-closed). Kapatmak için
değişkeni boş bırakın.

## 8. İzleme
- `GET /healthz` — süreç canlı.
- `GET /readyz` — DB erişilebilir (docker healthcheck ve nginx bunu kullanır).
- 5xx/panik → log (birincil) + opsiyonel webhook. Logları `make logs` ile izleyin.

## 9. Yedekleme ve DR (kritik)
- Cron'u kurun: `deploy/backup/crontab.example` (günlük dump + offsite + WAL).
- Aylık **restore testi**: `make restore-test` (sadece PG).
- Aylık/sürüm öncesi **tam DR tatbikatı**: `make dr-drill` (PG + MinIO).
- Tatbikat sonucunu `docs/runbook-dr.md` kayıt tablosuna işleyin — Plan §5.3
  gereği "test edilmeyen yedek, yedek değildir".
- Başarısız yedek otomatik uyarı üretir; uyarıyı ciddiye alın.

## 10. Performans
- Yük testi: `make loadtest` (100 eşzamanlı kullanıcı; `deploy/loadtest/README.md`).
- İndeks/N+1 analizi: `docs/faz10-performans-nplus1.md`.
- Yavaşlık şüphesinde `EXPLAIN (ANALYZE, BUFFERS)` ile sıcak sorguları inceleyin.

## 11. Sürüm yükseltme
1. `make dr-drill` (güvenlik ağı).
2. Yeni kodu çekin, `make migrate-up` (idempotent).
3. `docker compose ... up -d --build`.
4. `/api/v1/meta` sürümünü ve smoke testi (`make smoke`) doğrulayın.
