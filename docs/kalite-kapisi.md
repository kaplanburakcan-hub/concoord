# Kalite Kapısı (CI) — Geliştirme Planı Faz A

Bu doküman `docs/gelistirme-plani.md` içindeki **1. Ürün Kalitesi ve Otomatik
Kalite Kapısı** maddesinin uygulamasını anlatır.

> **Kabul kriteri:** Bir PR; backend testleri, frontend build'i, migration
> testi ve kritik smoke testler geçmeden `main`'e merge edilememeli.

## CI işleri (`.github/workflows/ci.yml`)

Her pull request'te ve `main`'e her push'ta çalışır.

| İş | Kontroller |
|---|---|
| **Backend** | `gofmt` (biçimlendirilmemiş dosya varsa kırılır), `go vet`, RBAC yetki matrisi (ayrı adım, `-v`), `go test -race ./...` |
| **Frontend** | `npm ci`, `npm run typecheck` (tsc), `npm run lint` (ESLint), `npm test` (Vitest), `npm run build` |
| **Migration round-trip + API smoke** | Temiz PostgreSQL 16 → `migrate up` → `down -all` → `up`; ardından API build + seed + başlatma → `deploy/ci/api-smoke.sh` |
| **Docker image build** | `backend/` ve `frontend/` Dockerfile'larının build'i (push yok, GHA cache) |
| **Quality Gate** | Yukarıdaki dört işin hepsi başarılıysa geçer. Branch protection'da zorunlu kontrol olarak **yalnızca bu** seçilir. |

### Migration smoke testi (`deploy/ci/migration-smoke.sh`)

1. Her `NNNNNN_*.up.sql` için `.down.sql` var (ve tersi), sürüm numaraları benzersiz.
2. `up` tüm migration'ları uygular, son sürüm = en büyük dosya numarası, `dirty=false`.
3. `down -all` sonrası `public` şemasında `schema_migrations` dışında tablo,
   enum veya domain kalmaz.
4. İkinci `up` yine hatasız uygulanır. Böylece down migration'ların gerçekten
   tersine çevirdiği doğrulanır.

### API smoke + kontrat testi (`deploy/ci/api-smoke.sh`)

- `/healthz`, `/readyz`, `/api/v1/meta` yanıt şekilleri.
- Hata zarfı kontratı: `{"error":{"code","message","request_id"}}`.
- Yanlış parola → 401, doğru parola → `tokens.access_token` / `refresh_token`.
- Kimliksiz `/auth/me` → 401.
- Admin için kritik liste uçları (`/projects`, `/admin/users|roles|permissions|audit-logs`) → 200.
- **RBAC uçtan uca:** admin rolsüz bir test kullanıcısı oluşturur. Bu kullanıcı
  `/admin/users`, `/admin/audit-logs` ve `POST /projects` uçlarında 403 almalıdır.

### Frontend testleri

| Dosya | Kapsam |
|---|---|
| `src/api/client.test.ts` | Token saklama, 401 → tek seferlik refresh + istek yenileme, refresh başarısızsa oturum düşürme, hata zarfı → `RequestError`, `describeLoadError` |
| `src/offline/queue.test.ts` | PWA offline kuyruk: çevrimdışı kuyruklama, sıra koruma, `{local:id}` yer tutucusu, 409 conflict, 5xx'te bekletme |
| `src/auth/guards.test.tsx` | `Can`, `RequirePerm` (403 ekranı), `RequireAuth` (`/login` yönlendirmesi) |

ESLint kuralları bilinçli olarak dar tutuldu. Gerçek hata üreten kalıplar
(`rules-of-hooks`, `prefer-const`, tanımsız değişken vb.) **error** seviyesinde
ve CI'ı kırar. `any` kullanımı ve kullanılmayan değişken gibi temizlik
önerileri ise **warning** seviyesinde; mevcut uyarılar kademeli temizlenecek.

## Yerelde çalıştırma

```bash
make ci                 # gofmt/vet, go test -race, typecheck, lint, vitest, build

# Migration round-trip (BOŞ bir veritabanı gerekir)
IPKS_DB_DSN='postgres://ipks:ipks@localhost:5432/ipks_ci?sslmode=disable' make migrate-smoke

# Çalışan API'ye karşı smoke testi
BASE_URL=http://localhost:8080 ADMIN_EMAIL=admin@ipks.local ADMIN_PASS=... make api-smoke
```

## Branch protection (bir kerelik, repo yöneticisi yapar)

Kabul kriterinin "merge edilememeli" kısmı GitHub ayarıyla sağlanır. Workflow
dosyası bunu kendisi zorlayamaz.

1. GitHub → **Settings → Branches → Add branch ruleset** (veya *Branch protection rule*).
2. Hedef: `main`.
3. Şunları işaretleyin:
   - **Require a pull request before merging**
   - **Require status checks to pass** → aranacak kontrol: **`Quality Gate`**
     (CI en az bir kez çalıştıktan sonra listede görünür)
   - **Require branches to be up to date before merging** (önerilir)
   - **Block force pushes**
4. İsterseniz **Do not allow bypassing the above settings** ile yöneticiler de kurala dahil edilir.

Bundan sonra `Quality Gate` kırmızıyken PR merge edilemez ve `main`'e doğrudan push yapılamaz.
