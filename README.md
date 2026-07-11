# İPKS — Faz 0: Temel Altyapı

İnşaat Proje Koordinasyon ve Saha Takip Platformu monorepo iskeleti.
Plan referansı: `insaat-pys-program-plani-v1_1.md` (Bölüm 8, Faz 0).

## İçerik (Faz 0 kapsamı)

| Kalem | Nerede |
|---|---|
| Monorepo iskeleti | `backend/` (Go), `frontend/` (React/Vite/TS/Tailwind), `deploy/`, `docs/` |
| Docker Compose (api + worker + postgres + minio) | `deploy/docker-compose.yml` |
| Migration altyapısı (golang-migrate, up/down) | `backend/migrations/`, `make migrate-up / migrate-down` |
| Konfigürasyon yönetimi (env tabanlı) | `backend/internal/config`, `.env.example` |
| Yapılandırılmış loglama (slog, prod'da JSON) | `backend/internal/logger` |
| API error envelope | `backend/internal/httpx/envelope.go` |
| Audit-log middleware iskeleti + tabloları | `backend/internal/audit`, migration `000001` |
| Seed mekanizması (idempotent, `seed_history`) | `backend/internal/seed`, `make seed` |
| VPS deploy (nginx + Let's Encrypt) | `deploy/docker-compose.prod.yml`, `deploy/nginx/`, `deploy/scripts/` |
| Offsite yedek + restore testi | `deploy/backup/backup.sh`, `restore-test.sh`, `crontab.example` |

## Hızlı başlangıç (geliştirme)

```bash
cp .env.example .env        # parolaları değiştirin
make up                     # postgres + minio + api + worker + frontend
make migrate-up             # şema
make seed                   # idempotent seed
curl localhost:8080/readyz  # {"status":"ready"}
# Arayüz: http://localhost:5173  → Faz 0 sistem durum ekranı
```

## VPS'e yayına alma

```bash
sudo bash deploy/scripts/vps-setup.sh      # docker + ufw
cp .env.example .env                       # DOMAIN, parolalar, OFFSITE_S3_* doldurun
bash deploy/scripts/init-letsencrypt.sh    # ilk HTTPS sertifikası
bash deploy/scripts/deploy.sh              # build + migrate + up + sağlık kontrolü
crontab -e                                 # deploy/backup/crontab.example içeriğini ekleyin
```

## Kabul kriterleri karşılığı (Plan, Faz 0)

- **`docker compose up` ile sağlıklı sistem** → tüm servislerde healthcheck; `readyz` DB'yi doğrular.
- **Migration up/down çalışıyor** → `make migrate-up` / `make migrate-down` (golang-migrate).
- **Günlük offsite yedek cron'u + restore testi** → `backup.sh` (30 gün günlük + 12 ay aylık, dump + WAL + MinIO mirror), `restore-test.sh` ayrı container'da geri yükleyip raporlar; cron örneği hazır.
- **VPS'te HTTPS erişim** → nginx + certbot (otomatik yenileme, 12 saatte bir).

---

# Faz 1 — Kimlik Doğrulama ve Yetki Motoru

JWT auth + RBAC (kullanıcı bazlı override) + Admin Paneli v1. Audit ve seed
altyapısı bu fazda ilk gerçek tüketicilerini aldı.

## İçerik (Faz 1 kapsamı)

| Kalem | Nerede |
|---|---|
| JWT auth (login/refresh/logout, şifre sıfırlama/değiştirme) | `backend/internal/auth` |
| argon2id parola özeti | `backend/internal/auth/password.go` |
| İzin sözlüğü + 7 rol + rol varsayılanları (tek kaynak) | `backend/internal/rbac/defaults.go` |
| Yetki motoru (DENY > GRANT > rol) — saf çekirdek + DB adaptörü | `backend/internal/rbac/engine.go` |
| `RequirePermission(module.action)` + `Authenticate` middleware | `backend/internal/auth/middleware.go` |
| Admin Paneli v1 API (kullanıcı CRUD, matris, rol atama, audit) | `backend/internal/admin` |
| Seed: izin/rol/rol-izin + bootstrap admin (idempotent) | `backend/internal/seed` |
| Frontend `<Can>` + route guard + izin matrisi + audit görüntüleyici | `frontend/src/auth`, `frontend/src/pages` |
| Şema: users/roles/permissions/user_permissions/project_members + token tabloları | migration `000002` |
| Otomatik yetki test matrisi (rol × kritik endpoint) | `backend/internal/rbac/engine_test.go` |
| API sözleşmesi | `docs/api/faz1-auth-rbac.md` |

## Hızlı başlangıç (Faz 1)

```bash
cp .env.example .env        # IPKS_JWT_SECRET ve parolaları ayarlayın
make up                     # servisler
make migrate-up             # 000001 + 000002 şema
make seed                   # izin sözlüğü + 7 rol + bootstrap admin
                            # (parola boşsa üretilen parola api loglarında görünür: make logs)
make test                   # birim testler + yetki matrisi (yeşil olmalı)
# Arayüz: http://localhost:5173  → giriş → Panel + Admin Paneli
```

Bootstrap admin varsayılan e-postası `admin@ipks.local`. Parola `.env`'de
`IPKS_BOOTSTRAP_ADMIN_PASSWORD` ile verilmezse seed rastgele üretip **uyarı
logu** olarak yazar; `make logs` ile görüp ilk girişte değiştirin.

## Kabul kriterleri karşılığı (Plan, Faz 1)

- **7 rol seed'li** → `internal/rbac` tek kaynaktan seed edilir; `make seed`.
- **Admin tek bir izni override edebiliyor, etki anında API'de doğrulanıyor** →
  `PUT /admin/users/{id}/permissions/{code}` sonrası motor DENY>GRANT>rol'ü
  yeniden uygular; `docs/api/faz1-auth-rbac.md` sonundaki örnek akış.
- **Otomatik yetki test matrisi (rol × kritik endpoint) yeşil** →
  `make test` (`TestRoleEndpointAuthorizationMatrix` + `TestDecidePrecedence`).

## Sonraki adım (Faz 1'den)
Faz 2 — Proje çekirdeği + doküman altyapısı (aşağıda).

---

# Faz 2 — Proje Çekirdeği + Doküman Altyapısı

Plan referansı: Bölüm 8 (Faz 2), §3, §5.2, §6.2.

## İçerik (Faz 2 kapsamı)

| Kalem | Nerede |
|---|---|
| Proje CRUD + künye | `backend/internal/projects`, `frontend/.../pages/projects` |
| Milestone yönetimi | aynı paket; `projects/{id}/milestones` uçları |
| Çoklu proje navigasyonu (proje seçici) | `frontend/src/projects/ProjectContext.tsx`, `components/AppShell.tsx` |
| Doküman motoru: klasör ağacı, versiyonlama, SHA-256, kategori, polimorfik bağ | `backend/internal/documents`, migration `000003` |
| MinIO entegrasyonu (stdlib SigV4; yeni bağımlılık yok) | `backend/internal/storage/s3.go` |
| Sözleşme/zeyilname arşivi (ilk tüketici) | `doc_category=Contract/Addendum`, `frontend/.../pages/documents` |
| API sözleşmesi + ADR | `docs/api/faz2-projeler-dokumanlar.md`, `docs/adr/0003-faz2-proje-dokuman.md` |

## Hızlı başlangıç (Faz 2)

```bash
make up && make migrate-up && make seed   # 000003 migration + izin sync dahil
# admin ile giriş → "Projeler"den proje aç → üst bardaki seçiciyle projeler arası geçiş
# "Dokümanlar" → klasör oluştur → doküman ekle → dosya yükle (v1) → tekrar yükle (v2)
```

Not: `make seed` mevcut kurulumda yeni `projects.create/delete` izinlerini
idempotent uygular (adım `0006_faz2_izin_sync`).

## Kabul kriterleri karşılığı (Plan, Faz 2)

- **İki ayrı projede aynı kullanıcı farklı rollerle** → `project_members.role_id`
  proje bazlı; seçici proje değişince izinler `/auth/me?project_id=` ile yeniden çözülür.
- **Sözleşme PDF'i v1→v2 versiyonlanıyor, eski versiyon indirilebiliyor** →
  `POST .../documents/{id}/versions` her yüklemede `v{n}` üretir; tüm versiyonlar
  SHA-256'lı ve indirilebilir (append-only).
- **Yetkisiz kullanıcı URL'i bilse dahi dosyaya erişemiyor** → yükleme/indirme
  API üzerinden geçer, `documents.*` izni + proje kapsamı zorunlu; depolama anahtarı
  istemciye dönmez.

## Faz 3 — Taşeron ve Hakediş Yönetimi (Finansal Çekirdek)

Platformun finansal kayıt sistemi: taşeron kartları, alt sözleşme arşivi, birim
fiyat cetveli (BOQ) ve kümülatif hakediş iş akışı.

| Ne | Nerede |
|---|---|
| Kümülatif hesap çekirdeği (saf, **birim testli**, Plan §6.4) | `backend/internal/payments/calc.go`, `calc_test.go` |
| Hakediş iş akışı + kesinti + finansal maskeleme | `backend/internal/payments/payments.go`, `subcontractors.go` |
| Finalized **DB kilidi** (trigger) + şema | `backend/migrations/000004_taseron_ve_hakedis.up.sql` |
| Excel/CSV içe aktarma (stdlib; yeni bağımlılık yok) | `backend/internal/payments/import.go` |
| Hakediş özet PDF'i (stdlib) | `backend/internal/payments/pdf.go` |
| Ön yüz (taşeron/metraj/hakediş) | `frontend/src/pages/payments/` |
| API sözleşmesi + ADR | `docs/api/faz3-taseron-hakedis.md`, `docs/adr/0004-faz3-taseron-hakedis.md` |

### Hızlı başlangıç (Faz 3)

```bash
make up && make migrate-up   # 000004 migration (tablolar + Finalized kilit trigger'ları)
cd backend && go test ./internal/payments/...   # sentetik doğrulama seti dahil birim testler
# admin ile giriş → "Taşeronlar" → firma ekle → birim fiyat cetvelini elle/Excel ile gir
# "Hakedişler" → yeni taslak → metraj gir (Kaydet & Hesapla) → Saha Onayına Gönder →
#   Saha Onayı Ver → Kesinleştir (kilitlenir) → Özet PDF
```

Yeni izin gerekmez: taşeron/sözleşme/BOQ `contracts.*`, hakediş `progress_payments.*`
izinlerini kullanır. `progress_payments.view_financials` metraj ile tutar görünürlüğünü
ayırır; `project_members.subcontractor_id` satır seviyesi güvenliği sağlar.

### Kabul kriterleri karşılığı (Plan, Faz 3)

- **Uçtan uca**: taşeron metraj girer (`edit_draft`) → saha müh. onaylar (`approve`)
  → PY kesinleştirir (`finalize`) → kayıt DB trigger ile **kilitlenir** (sonraki
  UPDATE/DELETE 409).
- **Kümülatif 2. dönemde doğru taşınıyor + avans/teminat birebir** →
  `calc_test.go::TestSyntheticTwoPeriods` sentetik seti elle hesaplanan değerlerle
  doğrular (avans mahsubu 2. dönemde kalan avansla sınırlanır).
- **Satır seviyesi güvenlik + view_financials** → SubcontractorRep yalnızca kendi
  taşeronunu görür; SiteEngineer onay verir ama tutarları görmez.

---

# Faz 4 — Görev Yönetimi + Bildirim Motoru

Plan referansı: Bölüm 8 (Faz 4), §6.5, §9. API sözleşmesi:
`docs/api/faz4-gorevler-bildirimler.md` · Kararlar: `docs/adr/0005-faz4-gorev-bildirim.md`

## İçerik (Faz 4 kapsamı)

- **Merkezi bildirim motoru** (`internal/notify`): in-app kayıt + kanal
  tercihlerine göre e-posta (SMTP) ve SMS (TR sağlayıcı adaptörü — Netgsm hazır,
  arayüz soyutlanmış) işlerinin kuyruğa yazılması. Sonraki tüm modüller bu
  servisi kullanır.
- **PostgreSQL iş kuyruğu** (`job_queue`): worker `FOR UPDATE SKIP LOCKED` ile
  işleri çeker; başarısızlıkta üstel geri çekilme (en çok 5 deneme). Redis yok.
- **Worker** artık gerçek iş işler: `send_email` / `send_sms` + 15 dakikada bir
  **deadline hatırlatıcı** taraması (termin ≤ 24 saat, Done değil, görev başına
  tek hatırlatma; termin değişirse yeniden kurulur).
- **Kanban görev panosu**: sürükle-bırak (float `kanban_order`), görev CRUD,
  öncelik/termin, atama (`tasks.assign`), `edit_own`/`edit_all` ayrımı, statü
  geçişleri `workflow_transitions`ta, yazımlar audit'li, optimistic locking.
- **Yorum + @mention**: `@kullanıcıadı` proje üyeleri arasında çözümlenir;
  bahsedilenlere `task_mention`, diğer ilgililere `task_comment` bildirimi.
- **Arayüz**: /gorevler Kanban sayfası, üst barda bildirim zili (okunmamış
  sayaç + açılır liste + okundu işaretleme), /bildirim-ayarlari kanal tercihleri.

## Hızlı başlangıç (Faz 4)

```bash
make migrate-up          # 000005_gorev_ve_bildirim
make up                  # api + worker + frontend
# SMTP için .env: IPKS_SMTP_HOST/PORT/USER/PASS/FROM (boşsa e-posta loglanır)
# SMS için .env: IPKS_SMS_PROVIDER=netgsm + IPKS_SMS_USER/PASS/HEADER
go test ./internal/notify/... ./internal/tasks/...   # mention + backoff + doğrulama testleri
```

## Kabul kriterleri karşılığı (Plan, Faz 4)

- **"Görev atanan kullanıcı in-app + e-posta bildirimi alıyor"** → atama anında
  in-app `task_assigned` kaydı ve `send_email` kuyruk işi oluşur; worker SMTP
  üzerinden gönderir (`sent_at` işlenir). Zilde okunmamış sayaç anında artar.
- **"Deadline yaklaşınca otomatik hatırlatma üretiliyor"** → worker açılışta ve
  15 dakikada bir tarar; `task_deadline` bildirimi atananlara (yoksa oluşturana)
  tüm tercihli kanallardan gider.

---

# Faz 5 — Malzeme Onay Süreci (Submittals / MAR)

Plan referansı: Bölüm 8 (Faz 5), §6.5. API sözleşmesi:
`docs/api/faz5-malzeme-onayi.md` · Kararlar: `docs/adr/0006-faz5-malzeme-onayi.md`

## İçerik (Faz 5 kapsamı)

- **MAR formu + statü akışı**: `Submitted → UnderReview → Approved |
  ConditionallyApproved | Rejected`; proje içi sıralı `MAR-xxx` numaralama
  (advisory lock + tekil indeks), geçişler `workflow_transitions`ta, yazımlar
  audit'li (migration `000006_malzeme_onayi`, paket `internal/materials`).
- **Karar notu zorunluluğu**: uygulama doğrulaması + DB CHECK kısıtı — karara
  bağlanmış satırda not boş olamaz; `decided_by/decided_at` işlenir.
- **Doküman ekleri**: Faz 2 polimorfik motor (`entity_type='material_approval'`,
  kategori `Submittal`); versiyonlama/SHA-256 hazır gelir.
- **Kısıtlı inceleme ekranı (Client)**: müşavir/işveren yalnızca *kendisine
  sunulan* (UnderReview ve karara bağlanmış) MAR'ları görür — filtre liste,
  tekil kayıt ve CSV'de backend'de zorunludur. SubcontractorRep yalnızca kendi
  firmasının kayıtlarını görür ve kendi firması adına MAR açabilir.
- **Renk kodlu durum panosu**: /malzeme-onaylari — statü kolonlu pano; detay
  sayfasında ek yükleme/indirme, incelemeye alma ve karar aksiyonları.
- **MAR kayıt defteri dışa aktarımı**: `register.csv` (`;` ayırıcı + UTF-8 BOM,
  TR Excel uyumlu), kapsam filtreli.
- **Bildirimler (Faz 4 motoru)**: `mar_submitted` → inceleyiciler;
  `mar_under_review` → karar vericiler (Client dahil); `mar_decided` → talep
  sahibi + taşeron temsilcileri.

## Hızlı başlangıç (Faz 5)

```bash
make migrate-up          # 000006_malzeme_onayi
make up
go test ./internal/materials/...   # karar notu + kapsam doğrulama testleri
# Arayüz: /malzeme-onaylari (pano) → kart → detay (ekler, inceleme, karar)
```

## Kabul kriterleri karşılığı (Plan, Faz 5)

- **"Client rolü yalnızca kendisine sunulan MAR'ları görüp karar verebiliyor"**
  → Client kapsam filtresi backend'de zorunlu (Submitted görünmez, id bilinse
  dahi 404); `material_approvals.decide` Client rol varsayılanında, karar ucu
  yalnızca UnderReview kayıtlarda çalışır ve karar notu ister.
- **"Karar bildirimi ilgililere düşüyor"** → karar anında talep sahibi + ilgili
  taşeronun temsilcilerine `mar_decided` bildirimi (in-app + tercihe göre
  e-posta/SMS) merkezi notify servisiyle gönderilir.

## Sonraki adım
Faz 6 — Günlük/Haftalık Saha Raporlama. Haftalık PDF derleyici MAR özetlerini
(bekleyen MAR sayısı/yaşlandırma) bu fazın tablosundan okuyacak.

# Faz 7 — Satınalma ve Tedarik Zinciri

Plan referansı: Bölüm 6.6 ve 8 (Faz 7). ADR: `docs/adr/0008-faz7-satinalma.md`,
API sözleşmesi: `docs/api/faz7-satinalma.md`.

## İçerik (Faz 7 kapsamı)

- **PR formu + onay akışı**: `Draft → Submitted → Approved | Rejected`;
  ret gerekçesi zorunlu. Draft dışı kalemler ve karara bağlanmış başlık
  **DB trigger** kilidiyle değişmez (migration `000008_satinalma`, paket
  `internal/procurement`). Numaralama proje içinde sıralı (PR-001/PO-001,
  advisory lock).
- **PR→PO dönüşümü**: tek transaction — PR `Converted`, yeni PO `pr_id`
  bağıyla açılır; zincir `workflow_transitions` + `po.pr_id` üzerinden uçtan
  uca izlenebilir. PR'sız acil alım için bağımsız PO ucu vardır.
- **PO durum takibi + kısmi teslimat**: statü teslimat kayıtlarından türer
  (`PartiallyDelivered` / `mark_delivered` ile `Delivered`); iptal ayrı uç.
  Kapanmış sipariş düzenlenemez, teslimat alamaz.
- **İrsaliye fotoğrafı (mobil kamera)**: Faz 2 doküman motoru
  (`doc_category='Delivery'`) + `deliveries.document_id` bağı; arayüzde
  `capture="environment"` ile sahada doğrudan kamera.
- **Tedarik durum panosu**: /satinalma/siparisler — sayaçlar + geciken
  PR/PO vurgusu (kırmızı). Worker saatlik taramayla geciken PO için
  `po_overdue` bildirimi üretir (`overdue_notified_at` ile tekrarsız; tarih
  güncellenirse uyarı yeniden kurulur).
- **Bildirimler (Faz 4 motoru)**: `pr_submitted` → onay yetkilileri;
  `pr_decided` → talep sahibi; `po_overdue` → sipariş sahibi + PR sahibi.

## Hızlı başlangıç (Faz 7)

```bash
make migrate-up          # 000008_satinalma
make up
go test ./internal/procurement/...   # PR/PO/teslimat doğrulama testleri
# Arayüz: /satinalma (talepler) → detay (onay akışı, dönüşüm)
#         /satinalma/siparisler (pano + PO) → detay (teslimat + irsaliye foto)
```

## Kabul kriterleri karşılığı (Plan, Faz 7)

- **"PR→PO→teslimat zinciri uçtan uca izlenebiliyor"** → `po.pr_id` +
  `deliveries.po_id` FK zinciri; dönüşüm tek tx; tüm geçişler
  `workflow_transitions`ta, tüm değişiklikler `audit_logs`ta.
- **"İhtiyaç tarihi geçmiş, teslim edilmemiş kalemler uyarı üretiyor"** →
  pano geciken PR/PO listesini canlı hesaplar (liste/detayda kırmızı rozet);
  worker geciken PO için ilgililere `po_overdue` bildirimi gönderir.

---

# Faz 9 — Dashboard, EVM ve Yönetim Raporlaması (0.10.0)

- **EVM motoru**: saf hesap çekirdeği (`internal/dashboard/evm.go`) + DB
  derleyicisi (`LoadEVM`). PV = dağılım (manuel giriş → milestone ağırlığı →
  doğrusal) × BAC; EV = kesin hakediş brütü oranı × BAC; AC = kesin hakediş
  net + teslim alınmış PO. SPI/CPI/EAC/ETC; payda 0 → tanımsız (0).
  Elle hesaplanmış kontrol setiyle birim testli (`evm_test.go`).
- **Rol duyarlı dashboard**: `GET /projects/{id}/dashboard` — ilerleme,
  milestone timeline, S-eğrisi + SPI/CPI kartları (yalnızca
  `reports.view_financial_reports`), açık İSG bulguları, bekleyen onaylar,
  aktivite akışı. Taşeron temsilcisi yalnızca kendi firmasının sayaçlarını
  görür. Portföy görünümü: `GET /portfolio` (/portfoy sekmesi).
- **PV aylık dağılım girişi**: `pv_plan_entries` (+ dashboard'da editör,
  `projects.edit`); toplam 100±0.5 doğrulanır.
- **Aylık yönetim raporu**: haftalık rapor deseni — snapshot senkron
  dondurulur, PDF worker'da (`monthly_report_pdf`) üretilir. İçerik: EVM +
  ay içinde kesinleşen hakedişler ve kesinti dökümü + milestone gerçekleşme
  + İSG performansı + tedarik özeti. Arayüz: /aylik-raporlar.
- **Eşik tabanlı uyarılar**: `project_control_settings` (CPI/SPI alt eşiği,
  bulgu yaşlanma günü) + `control_alerts` tekilleştirme defteri (ay bazlı).
  Worker 6 saatte bir tarar; ihlalde PY'lere bildirim.

## Hızlı başlangıç (Faz 9)

```bash
make migrate-up          # 000010_dashboard_evm
make up
go test ./internal/dashboard/...   # EVM kontrol seti + PDF duman testi
# Arayüz: / (rol duyarlı dashboard) · /portfoy · /aylik-raporlar
```

## Kabul kriterleri karşılığı (Plan, Faz 9)

- **"EVM değerleri elle hesaplanan kontrol setiyle birebir tutuyor"** →
  `evm_test.go`: BAC 1.000.000 senaryosunda SPI 0,833 / CPI 1,064 /
  EAC 940.000 / ETC 705.000 elle doğrulanmış beklenenlerle birebir.
- **"Aylık rapor tek tıkla üretiliyor"** → POST + worker PDF; rakamlar
  snapshot'tan doğrulanabilir; liste ekranı Pending→Ready'yi canlı izler.
- **"Eşik ihlali PY'ye otomatik bildirim üretiyor"** → CPI/SPI, geciken
  milestone, yaşlanan bulgu taraması; ay içinde tekrarsız.

Ayrıntı: `docs/api/faz9-dashboard-evm.md`, `docs/adr/0010-faz9-dashboard-evm.md`.

---

# Faz 10 — Sertleştirme ve Yayına Alma (1.0.0)

Son faz: yeni iş modülü eklemez, sistemi prod'a hazırlar. Kabul (Plan §8):
prod checklist eksiksiz + belgelenmiş DR tatbikatı geçti.

- **Güvenlik (çapraz kesen middleware):** İstek hız sınırlama — genel token-bucket
  (`IPKS_RATE_LIMIT_RPS`/`_BURST`, IP başına) + kimlik uçlarında sıkı sabit-pencere
  (`IPKS_LOGIN_RATE_LIMIT`, kaba kuvvete karşı); CORS allowlist (joker yok);
  güvenlik başlıkları (nosniff, X-Frame-Options, Referrer-Policy, COOP).
  429 için yeni hata kodu `rate_limited`. `internal/httpx/{ratelimit,security}.go`.
- **Yükleme güvenliği:** Dosya tipi doğrulama — tarayıcının `Content-Type`'ına
  güvenilmez; ilk 512 bayt koklanır (magic byte) ve uzantı allowlist'i ile çapraz
  kontrol edilir, yürütülebilir içerik reddedilir. Opsiyonel ClamAV taraması
  (`IPKS_CLAMD_ADDR`, fail-closed). `internal/storage/filecheck.go`; doküman ve
  İSG foto yükleme yollarına bağlı.
- **Performans:** Ek indeksler (`000011_faz10_performance`: EVM finalized yolu,
  ceza→kesinti köprüsü, versiyon join'i). Index/N+1 analizi:
  `docs/faz10-performans-nplus1.md`. Yük testi: `deploy/loadtest/` (k6, 100 VU).
- **İzleme:** `/healthz`+`/readyz` (mevcut) + 5xx/panik → opsiyonel webhook
  (`IPKS_ERROR_WEBHOOK_URL`, asenkron best-effort). `internal/httpx/errorreport.go`.
- **DR tatbikatı:** `deploy/backup/dr-drill.sh` — offsite'tan PostgreSQL + MinIO
  restore'u geçici container'larda doğrular, zaman damgalı rapor üretir; prod'a
  dokunmaz. Runbook: `docs/runbook-dr.md`.
- **PWA cilası:** Sürümlü service worker (v2), çevrimdışı yedek sayfası
  (`/offline.html`), yeni-sürüm devralma akışı.
- **Belgeler:** `docs/kilavuz-kullanici.md`, `docs/kilavuz-admin.md`,
  `docs/prod-go-live-checklist.md`, `docs/devir-teslim.md`.

## Hızlı başlangıç (Faz 10)

```bash
make migrate-up          # 000011_faz10_performance
make up
make smoke               # k6 duman testi (BASE_URL/IPKS_USER/IPKS_PASS)
make loadtest            # 100 eşzamanlı kullanıcı
make dr-drill            # tam DR tatbikatı (PG + MinIO); runbook'a işle
```

## Kabul kriterleri karşılığı (Plan, Faz 10)

- **"Prod checklist eksiksiz"** → `docs/prod-go-live-checklist.md` (altyapı,
  güvenlik, performans, dayanıklılık, izleme, PWA, uçtan uca duman testi, belgeler).
- **"Restore tatbikatı belgelenmiş şekilde geçildi"** → `make dr-drill` +
  `docs/runbook-dr.md` kayıt tablosu.

Ayrıntı: `docs/api/faz10-sertlestirme.md`, `docs/adr/0011-faz10-sertlestirme-yayina-alma.md`.
