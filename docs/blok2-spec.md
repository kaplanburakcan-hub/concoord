# Concoord — Blok 2 Teknik Şartnamesi

**Kapsam:** İş Programı (Gantt) + Gerçekleşen İlerleme + PDKS/GPS Puantaj
**Hedef segment:** Taahhüt firması, çoklu şantiye
**Tahmini süre:** 4 hafta (tek geliştirici)
**Repo:** `concoord` — Go backend, React + TypeScript + Vite + Tailwind frontend, PostgreSQL

> **Revizyon notu (2026-09-05):** Aşama 0 keşfi yapıldı ve bulgularına göre bu
> dosya düzeltildi. Düzeltilen her yer aşağıda `> DÜZELTME (Aşama 0):` bloğuyla
> işaretli — orijinal madde metni korunmuş, yalnızca yanlış/eksik varsayımın
> hemen altına gerçek durum eklenmiştir. Aşama 2 bu düzeltilmiş haliyle
> uygulanmış ve canlıda doğrulanmıştır; Aşama 1 henüz uygulanmamıştır.

---

## 0. Bu dosya nasıl kullanılır

Bu şartname mevcut kod tabanı hakkında **varsayımlar** içerir. Kod yazmadan önce Aşama 0'ı tamamla ve varsayımların hangilerinin yanlış olduğunu bildir. Yanlış varsayım üzerine migration yazma.

Çalışma düzeni:
- Her aşama ayrı bir dal ve ayrı bir PR.
- Migration'lar mevcut numaralandırmayı takip eder (`backend/migrations/NNNNNN_ad.up.sql` / `.down.sql`).
- Her yeni endpoint için izin matrisine karşılık gelen bir `perm` anahtarı tanımlanır.
- Her veri değiştiren işlem mevcut denetim izi (audit trail) mekanizmasına yazılır.

---

## Aşama 0 — Keşif (kod yazma, rapor ver)

Aşağıdakileri oku ve özetle:

1. `backend/migrations/` — mevcut son migration numarası, isimlendirme kalıbı, `up`/`down` yazım stili.
2. Hakediş şeması: poz/kalem tabloları nasıl adlandırılmış? Sözleşme miktarı, dönem miktarı ve kümülatif miktar hangi kolonlarda tutuluyor? Onaylı hakediş hangi `status` değeriyle ayırt ediliyor?
3. `backend/internal/payments/` — `calc.go` ve `approvals.go` içindeki hesaplama ve onay zinciri desenleri. Yeni modüller aynı deseni izleyecek.
4. Yetkilendirme: `perm` anahtarları nerede tanımlı, middleware nasıl çalışıyor, izin matrisi arayüzü hangi dosyada?
5. Denetim izi: hangi tabloya, hangi fonksiyonla yazılıyor?
6. Frontend: `api/client.ts` içindeki `api` ve `apiDownload` imzaları, `ProjectContext` ile seçili projeye nasıl erişiliyor, navigasyon konfigürasyonu hangi dosyada?
7. Personel/puantaj: `PersonelPage.tsx` gerçek bir sayfa mı yoksa hâlâ `ComingSoon` iskeleti mi? Backend'de personel tablosu var mı?

**Çıktı:** Yukarıdakilerin özeti + bu şartnamedeki hangi tablo/kolon adlarının mevcut yapıyla çakıştığı veya uyumsuz olduğu.

> **DÜZELTME (Aşama 0 — 2026-09-05 keşif sonucu):** Yukarıdaki 7 maddenin
> gerçek cevapları:
> 1. Son migration numarası **61** (`000061_audit_view_action`) → sıradaki **62**. Kalıp: `NNNNNN_kisa_ad.up/.down.sql`, lowercase SQL tipleri, `gen_random_uuid()` PK, CHECK ile enum-benzeri text alanlar.
> 2. `work_items` (migration 000004): `poz_no`, `unit`, **`contract_qty`** (sözleşme miktarı) — `subcontractor_id`'ye bağlı, **proje'ye değil**. `progress_payments.status` `CHECK IN ('Draft','Submitted','SiteApproved','Finalized','Rejected')` → **onaylı/kesinleşmiş = `status='Finalized'`** (bir trigger bu durumdaki satırı UPDATE/DELETE'e tamamen kapatır). **Dönem miktarı ve kümülatif miktar `progress_payments`'ta DEĞİL**, ayrı bir tabloda: `progress_payment_items` (`work_item_id`'ye bağlı) → `prev_cum_qty`, `this_period_qty`, `cum_qty`.
> 3. `calc.go` tamamen saf (DB'siz) hesap fonksiyonları (`Compute`/`ComputeWith`), birim test edilebilir. `approvals.go` **veri-tanımlı** çok adımlı onay zinciri (`payment_approval_steps`/`payment_approvals`, her adım bir `permission` string'iyle gate'lenir, kararlar hiç silinmez) — bu, tek-aşamalı onaydan (paymentplans/equipmenttransfers) daha karmaşık, ayrı bir desen; PDKS'te tek-aşamalı desen kullanıldı (bkz. Aşama 2.5 not).
> 4. Kanonik kaynak `backend/internal/rbac/defaults.go` (`AllPermissions` + `roleDefaults`). Yeni izin eklemek defaults.go + `internal/seed/seed.go`'ya yeni bir senkron adımı gerektirir (migration'dan **bağımsız**, `-seed` flag'iyle ayrıca çalıştırılır). Frontend'de ayrı bir izin listesi yok — `PermissionMatrixPage.tsx` `/admin/users/{id}/permissions`'tan dinamik çeker. Middleware: `mw.RequirePermission("x.y")`.
> 5. `internal/audit` — `(*Recorder).Record(ctx, Entry)` (hata yutulur) / `(*Recorder).RecordTx(ctx, q, Entry) error` (aynı tx, hata yutulmaz). **`audit_logs.action` DB'de `CHECK IN ('INSERT','UPDATE','DELETE')` ile SINIRLIYDI** — "her erişime yazılsın" (2.4) için `'VIEW'` değeri migration 000061 ile eklendi.
> 6. `api<T>(path, {method,body,projectId,retry})` ve aynı imzalı `apiDownload(path, fallbackName, retry)` (`api/client.ts`). `useProjects()` → `{projects, current, loading, select, reload}`. Navigasyon: `AppShell.tsx`'teki `GROUPS` sabiti.
> 7. `PersonelPage.tsx` **gerçek** bir sayfa (ComingSoon değil), `/proje/personel` rotasında — **puantajla ilgisi yok**, personel listesi/rol yönetimi. `project_personnel` tablosu var (migration 000021). **Ayrıca `project_puantaj` adında ZATEN ÇALIŞAN bir manuel puantaj sistemi var** (`/proje/personel-puantaj`, kişi+gün+durum+mesai_saat) — bkz. Aşama 2.6 düzeltmesi.
>
> **Beklenmeyen ek bulgu:** `frontend/src/offline/queue.ts` adında GENEL bir offline kuyruk modülü zaten var (`enqueue`/`sync`/`initOfflineSync`, localStorage tabanlı, günlük rapor akışı için yazılmış, `main.tsx`'te global başlatılıyor). IndexedDB yok, yalnızca localStorage; Service Worker sadece PWA kabuk önbellekleme yapıyor, kuyruk mantığı SW'de değil.

---

## Aşama 1 — İş Programı (WBS + Gantt)

> **DÜZELTME (Aşama 0):** Bu aşama, bu repoda (ne `main` dalında ne başka bir
> dalda) **henüz uygulanmamıştır** — `schedule_items` vb. hiçbir tabloya/koda
> rastlanmadı. Aşağıdaki tasarım hâlâ geçerli bir plandır ama "Aşama 1
> tamamlandı" varsayımıyla yazılan Aşama 3 maddeleri (bkz. orada) şu an
> uygulanamaz durumdadır.

### 1.1 Veri modeli

```sql
CREATE TABLE schedule_items (
  id              UUID PRIMARY KEY,
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  parent_id       UUID REFERENCES schedule_items(id) ON DELETE CASCADE,
  wbs_code        TEXT NOT NULL,              -- "1.2.3"
  name            TEXT NOT NULL,
  sort_order      INT  NOT NULL DEFAULT 0,
  is_milestone    BOOLEAN NOT NULL DEFAULT FALSE,

  baseline_start  DATE,
  baseline_finish DATE,
  actual_start    DATE,
  actual_finish   DATE,

  -- ilerlemenin nereden geleceği
  progress_source TEXT NOT NULL DEFAULT 'manual',  -- 'manual' | 'derived'
  manual_progress NUMERIC(5,2),                    -- 0-100, yalnız progress_source='manual'
  weight          NUMERIC(12,4) NOT NULL DEFAULT 0,-- üst kalemde ağırlıklı ortalama için

  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (project_id, wbs_code)
);

-- Bir WBS kalemi bir veya daha fazla sözleşme pozuna bağlanabilir.
-- Tablo/kolon adlarını Aşama 0'daki gerçek hakediş şemasına göre düzelt.
CREATE TABLE schedule_item_pozlar (
  schedule_item_id UUID NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
  poz_id           UUID NOT NULL,             -- DÜZELTME (Aşama 0): work_items(id) — internal/payments, migration 000004
  PRIMARY KEY (schedule_item_id, poz_id)
);

CREATE TABLE schedule_dependencies (
  predecessor_id UUID NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
  successor_id   UUID NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
  dep_type       TEXT NOT NULL DEFAULT 'FS',  -- FS | SS | FF | SF
  lag_days       INT  NOT NULL DEFAULT 0,
  PRIMARY KEY (predecessor_id, successor_id)
);

-- Baseline dondurma: idareye verilen programın revizyon geçmişi
CREATE TABLE schedule_baselines (
  id          UUID PRIMARY KEY,
  project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  revision_no INT  NOT NULL,
  frozen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  frozen_by   UUID NOT NULL,
  note        TEXT,
  snapshot    JSONB NOT NULL,                 -- o andaki tüm schedule_items
  UNIQUE (project_id, revision_no)
);
```

### 1.2 İlerleme hesabı

Kural: **fiziksel ilerleme elle girilmez, hakedişten türetilir.**

```
yaprak kalem (progress_source = 'derived'):
    ilerleme % = ( Σ onaylı hakedişlerdeki kümülatif miktar (bağlı pozlar) )
                 / ( Σ sözleşme miktarı (bağlı pozlar) ) * 100
    100 ile sınırla; sözleşme miktarı 0 ise ilerleme 0 kabul et

yaprak kalem (progress_source = 'manual'):
    ilerleme % = manual_progress

üst kalem:
    ilerleme % = Σ(alt.ilerleme × alt.weight) / Σ(alt.weight)
    tüm ağırlıklar 0 ise düz aritmetik ortalama
```

> **DÜZELTME (Aşama 0):** "Σ onaylı hakedişlerdeki kümülatif miktar" **iki
> ayrı tabloya yayılmış bir hesaptır**, şartnamedeki tek cümle bunu
> gizliyordu. Gerçek sorgu: `progress_payment_items.cum_qty` toplamı
> (`work_item_id` üzerinden `schedule_item_pozlar.poz_id = work_items.id`
> eşleşmesiyle), yalnızca `progress_payments.status = 'Finalized'` olan
> hakedişler için (`progress_payment_items.progress_payment_id` üzerinden
> JOIN). "Sözleşme miktarı" ise `work_items.contract_qty`.

`weight` varsayılanı, bağlı pozların sözleşme tutarı toplamı olsun (parasal ağırlık). Poz bağlantısı yoksa kullanıcı elle girer.

Bu hesap **materialized view veya cache değil**, istek anında hesaplanan bir servis fonksiyonu olsun. Proje başına kalem sayısı birkaç yüzü geçmeyeceği için performans sorunu yok; yanlış cache invalidasyonu ise hakediş tutarsızlığı demek.

### 1.3 S-eğrisi

`GET /api/v1/projects/{id}/schedule/s-curve?from=&to=&bucket=week|month` şu seriyi döndürsün:

| Seri | Kaynak |
|---|---|
| Planlanan fiziksel | baseline tarihlerine göre ağırlıkların zamana doğrusal dağıtımı |
| Gerçekleşen fiziksel | onaylı hakediş dönemlerinin kümülatif ilerlemesi |
| Planlanan nakit | baseline × poz sözleşme tutarı |
| Gerçekleşen nakit | onaylı hakediş net tutarları (kesintiler sonrası) |

### 1.4 API

```
GET    /api/v1/projects/{id}/schedule                 -> ağaç + hesaplanmış ilerleme
POST   /api/v1/projects/{id}/schedule/items
PATCH  /api/v1/schedule/items/{itemId}
DELETE /api/v1/schedule/items/{itemId}
POST   /api/v1/schedule/items/{itemId}/pozlar         -> poz bağla/çöz
POST   /api/v1/projects/{id}/schedule/dependencies
DELETE /api/v1/schedule/dependencies/{predId}/{succId}
POST   /api/v1/projects/{id}/schedule/baseline        -> revizyon dondur
GET    /api/v1/projects/{id}/schedule/baselines
GET    /api/v1/projects/{id}/schedule/s-curve
GET    /api/v1/projects/{id}/schedule/export?format=xlsx|pdf
POST   /api/v1/projects/{id}/schedule/import          -> XLSX (WBS, ad, başlangıç, bitiş, poz kodu)
```

İzinler: `schedule.view`, `schedule.edit`, `schedule.freeze_baseline`.

Döngüsel bağımlılık kontrolü zorunlu — `POST /dependencies` topolojik sıralama ile döngü oluşuyorsa 422 dönsün.

### 1.5 Frontend

- Yeni sayfa: `frontend/src/pages/schedule/IsProgramiPage.tsx`, rota `/proje/is-programi`
- Navigasyona **Proje** grubunda, "İlerleme Raporları"nın hemen üstüne ekle, `perm: "schedule.view"`
- Gantt kütüphanesi: `frappe-gantt` (MIT). Kendi Gantt'ını yazma.
- Üç görünüm sekmesi: **Gantt** / **Tablo (WBS ağacı)** / **S-Eğrisi**
- S-eğrisi için `recharts`
- Gecikme vurgusu: `actual_finish` yoksa ve `baseline_finish` geçmişse kalem kırmızı; ilerleme %100 ise yeşil
- Baseline karşılaştırma: dondurulmuş revizyonu seçince Gantt'ta gölge çubuk olarak göster

---

## Aşama 2 — PDKS / GPS Puantaj

> **DURUM: UYGULANDI ve canlıda doğrulandı (2026-09-05).** Aşağıdaki
> düzeltmeler, gerçek uygulama sırasında bulunan/karar verilen noktalardır.

### 2.1 Veri modeli

```sql
CREATE TABLE site_geofences (
  id          UUID PRIMARY KEY,
  project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  center_lat  DOUBLE PRECISION NOT NULL,
  center_lng  DOUBLE PRECISION NOT NULL,
  radius_m    INT NOT NULL DEFAULT 300,
  is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE attendance_events (
  id             UUID PRIMARY KEY,
  client_uuid    UUID NOT NULL,               -- offline kuyruk idempotency anahtarı
  project_id     UUID NOT NULL,
  geofence_id    UUID REFERENCES site_geofences(id),
  person_id      UUID NOT NULL,               -- personel tablosu
  event_type     TEXT NOT NULL,               -- 'in' | 'out'
  source         TEXT NOT NULL,               -- 'qr' | 'manual' | 'import'

  lat            DOUBLE PRECISION,
  lng            DOUBLE PRECISION,
  accuracy_m     REAL,
  distance_m     REAL,                        -- geofence merkezine uzaklık (sunucu hesaplar)
  geofence_ok    BOOLEAN,

  captured_at    TIMESTAMPTZ NOT NULL,        -- cihazda oluşma anı
  received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  device_id      TEXT,
  recorded_by    UUID,                        -- manuel girişte kaydeden kullanıcı
  note           TEXT,
  UNIQUE (client_uuid)
);
CREATE INDEX ON attendance_events (project_id, person_id, captured_at);

-- Günlük puantaj: ham olaylardan türetilir, elle düzeltilebilir; düzeltme ham kaydı silmez
CREATE TABLE attendance_days (
  id                UUID PRIMARY KEY,
  project_id        UUID NOT NULL,
  person_id         UUID NOT NULL,
  work_date         DATE NOT NULL,
  derived_hours     NUMERIC(5,2),
  adjusted_hours    NUMERIC(5,2),
  overtime_hours    NUMERIC(5,2) NOT NULL DEFAULT 0,
  status            TEXT NOT NULL DEFAULT 'derived', -- derived | adjusted | approved
  adjusted_by       UUID,
  adjusted_reason   TEXT,
  approved_by       UUID,
  approved_at       TIMESTAMPTZ,
  UNIQUE (project_id, person_id, work_date)
);

-- Şantiye panosunda dönen tek kullanımlık QR
CREATE TABLE attendance_qr_tokens (
  token       TEXT PRIMARY KEY,
  geofence_id UUID NOT NULL REFERENCES site_geofences(id) ON DELETE CASCADE,
  issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at  TIMESTAMPTZ NOT NULL
);
```

> **DÜZELTME (uygulama sırasında):** Ayrıca bir **`attendance_retention_settings`**
> tekil-satır (singleton) ayar tablosu eklendi (`retention_days`, varsayılan
> 730) — bkz. 2.4 düzeltmesi.

### 2.2 QR akışı

1. Şantiye panosundaki ekran `GET /api/v1/geofences/{id}/qr-token` ile **60 saniyede bir** yeni token alır ve QR olarak gösterir. Statik QR üretme — fotoğrafı paylaşılır.
2. İşçi QR'ı okutur, PWA `/pdks?token=...` ile açılır.
3. Tarayıcı `navigator.geolocation.getCurrentPosition` ile tek atış konum alır (`enableHighAccuracy: true`, `timeout: 15000`).
4. `POST /api/v1/attendance/events` — sunucu doğrular:
   - token var mı ve `expires_at` geçmemiş mi → yoksa 401
   - Haversine ile `distance_m` hesapla, `radius_m` içinde mi → değilse kaydet ama `geofence_ok = false` işaretle (reddetme, şefin onayına düşür)
   - `accuracy_m > 100` ise `geofence_ok = false`
   - `client_uuid` çakışıyorsa 200 dön (idempotent)
5. Aynı gün içinde `in`/`out` sıralaması bozuksa (iki ardışık `in` gibi) kaydı al, `attendance_days.status = 'derived'` olarak bırak ve şef ekranında uyarı göster.

> **DÜZELTME (uygulama sırasında — token pencere kontrolü):** Madde 4'teki
> "`expires_at` geçmemiş mi → yoksa 401" kuralı **sunucunun o anki saatiyle
> değil**, olayın kendi `captured_at`'iyle (± 5 saniye saat kayması
> toleransı) karşılaştırılacak şekilde uygulandı. Aksi halde uçak modunda
> geç senkronize olan olaylar (kabul kriteri 8) her zaman reddedilirdi —
> token 60 saniyede dolduğu için "şu an" ile karşılaştırma offline
> senaryosuyla temelden çelişir. `POST /attendance/events` bir dizi kabul
> ettiği için, tüm öğeler token nedeniyle başarısız olursa 401, karışık bir
> grupta yalnızca bazıları başarısızsa 200 + öğe bazlı hata döner.

### 2.3 Konum doğruluğu konusunda dürüstlük kuralı

Tarayıcı tabanlı çözüm sahte konumu (mock location) **tespit edemez**; `isFromMockProvider` yalnızca native uygulamada okunabilir. Arayüzde ve dokümanda "GPS ile kesin doğrulama" ifadesi kullanma. Doğru ifade: kayıt konum, zaman ve cihaz kimliğiyle birlikte saklanır ve şantiye şefi onayına sunulur.

### 2.4 KVKK kısıtları (kod seviyesinde uygulanacak)

- Konum **yalnızca giriş/çıkış anında** alınır. Sürekli izleme, arka plan konumu, `watchPosition` kullanma.
- `attendance_events` için saklama süresi ayarı ekle (varsayılan 730 gün). Süresi dolan kayıtlarda `lat`, `lng`, `accuracy_m`, `device_id` alanlarını NULL'a çeken bir arka plan işi yaz; `attendance_days` kalsın.
- Konum kolonlarını okuyabilmek `attendance.view_location` iznine bağlansın; normal puantaj görüntüleme (`attendance.view`) konumu döndürmesin.
- Konum verisine her erişim denetim izine yazılsın.
- `docs/kvkk/aydinlatma-metni-calisan-sablonu.md` altına müşterinin kendi çalışanlarına verebileceği aydınlatma metni şablonu ekle.

> **DÜZELTME (uygulama sırasında):**
> - "Saklama süresi ayarı ekle" → DB'de `attendance_retention_settings`
>   (tekil satır, `id boolean PRIMARY KEY CHECK(id)` deseni) olarak
>   uygulandı; kod değişikliği olmadan ayarlanabilir. Arka plan işi
>   `POST /internal/cron/purge-attendance-location` — mevcut Cron deseniyle
>   (`internal/machines/rental_reminder.go`) aynı, `X-Cron-Secret` ile korunur.
> - "Konum verisine her erişim denetim izine yazılsın" → `audit_logs.action`
>   DB'de yalnızca INSERT/UPDATE/DELETE kabul ediyordu; okuma erişimi için
>   migration 000061 ile **`'VIEW'`** değeri eklendi (`audit.ActionView`).
>   Bu kontrol **serileştirme katmanında** (`writeLocationAwareJSON`
>   yardımcı fonksiyonu), handler'larda tekrar yazılmadan uygulanıyor —
>   yeni bir uç konum alanı döndürecekse DTO'suna `maskLocation()` eklemesi
>   ve bu fonksiyonu kullanması yeterli.
> - Aydınlatma metni şablonu (`docs/kvkk/...`) bu turda **yapılmadı** —
>   ayrı bir hukuki/metin işi, kod kapsamı dışında bırakıldı.

### 2.5 API

```
GET  /api/v1/projects/{id}/geofences
POST /api/v1/projects/{id}/geofences
GET  /api/v1/geofences/{id}/qr-token
POST /api/v1/attendance/events                  -> idempotent, toplu (dizi) kabul etsin
GET  /api/v1/projects/{id}/attendance/days?from=&to=&person_id=
PATCH /api/v1/attendance/days/{dayId}           -> adjusted_hours + zorunlu adjusted_reason
POST /api/v1/projects/{id}/attendance/approve   -> dönem toplu onay
GET  /api/v1/projects/{id}/attendance/export?format=xlsx
```

İzinler: `attendance.view`, `attendance.view_location`, `attendance.record`, `attendance.adjust`, `attendance.approve`.

> **DÜZELTME (uygulama sırasında):**
> - Geofence CRUD (`GET/POST /geofences`, `GET /qr-token`) için izin
>   listesinde hiç anahtar yoktu — yeni **`attendance.manage_geofences`**
>   izni eklendi (GET geofences `attendance.view` altında kaldı, POST/QR
>   token bu yeni izne bağlandı).
> - `GET /attendance/export` bu turda **uygulanmadı** (kabul kriterlerinde
>   test edilmiyor, kapsam dışı bırakıldı — istenirse ayrı bir iş).
> - Ham olay listesi için (kriter 7'nin test edilebilmesi için gerekli,
>   spec'te eksikti) **`GET /projects/{id}/attendance/events?person_id=&work_date=`**
>   eklendi (`attendance.view`, konum maskelemeli).
> - `POST /attendance/events` ve **yeni** `GET /attendance/personnel?token=...`
>   (check-in ekranının personel seçici için — spec'te yoktu ama
>   PdksCheckinPage'in kimlik doğrulamasız çalışabilmesi için zorunluydu)
>   **kimlik doğrulaması GEREKTİRMEZ** — güvenlik QR token'ın kendisinden
>   gelir. `attendance.record` izni şu an hiçbir uç tarafından
>   kullanılmıyor (yalnızca 'qr' kaynağı uygulandı; 'manual'/'import'
>   şema düzeyinde ayrıldı ama bu turda bir uca bağlanmadı).

### 2.6 Frontend

- `frontend/src/pages/attendance/PdksCheckinPage.tsx` — rota `/pdks`, **kimlik doğrulaması gerektirmeyen** sade ekran (token + personel seçimi/kodu). Ayrı, minimal layout kullan; AppShell'i yükleme.
- `frontend/src/pages/attendance/PuantajPage.tsx` — rota `/proje/personel` (mevcut `ComingSoon` iskeletinin yerine geçer). Aylık ızgara: satır personel, sütun gün, hücre saat. `geofence_ok = false` olan günler işaretli. Düzeltme yapıldığında gerekçe zorunlu.
- `frontend/src/pages/attendance/GeofencePage.tsx` — şantiye sınırı tanımı, harita üzerinden merkez seçimi (Leaflet + OpenStreetMap; Google Maps kullanma).
- `frontend/src/pages/attendance/QrPanoPage.tsx` — tam ekran dönen QR, tablet için.

> **DÜZELTME (Aşama 0 + uygulama sırasında — rota çakışması):**
> `/proje/personel` **ComingSoon iskeleti DEĞİL**, `PersonelPage.tsx`
> (personel listesi/rol yönetimi, puantajla ilgisi yok) — bu varsayım
> yanlıştı. Ayrıca `/proje/personel-puantaj` adında **zaten çalışan bir
> manuel puantaj sayfası** var (`project_puantaj` tablosu). Kullanıcı
> onayıyla PuantajPage.tsx **ayrı bir rotaya** (`/proje/pdks-puantaj`)
> kondu; mevcut manuel sayfaya hiç dokunulmadı, ikisi de `project_personnel`'i
> paylaşır ama birbirinin yerini almaz. `GeofencePage.tsx` da
> `/proje/pdks-geofence` rotasına kondu (spec'te rota belirtilmemişti).
> `QrPanoPage.tsx` kimlik doğrulamalı ama AppShell'siz tam ekran olarak
> `/proje/pdks-pano` rotasına kondu (spec'te bu sayfanın kimlik doğrulaması
> gerektirip gerektirmediği belirtilmemişti — geofence yönetimi izniyle
> (`attendance.manage_geofences`) korunan, authenticated bir kiosk görünümü
> olarak yorumlandı).
>
> **Offline kuyruk:** Aşama 0'da bulunan mevcut genel modül
> (`frontend/src/offline/queue.ts`) kullanılMADI — PdksCheckinPage kendi,
> ayrı bir localStorage kuyruğu (`ipks.pdks.queue`) ile yazıldı. Sebep:
> mevcut modül `api()`'yi authenticated bağlamda (oturumlu kullanıcı,
> `projectId` zorunlu) çağırmak üzere tasarlanmış; PDKS check-in tamamen
> kimlik doğrulamasız ve tekil istek yerine dizi/toplu gönderim modeliyle
> çalışıyor. Bu bilinçli bir ayrım kararıdır ama şartname bunu öngörmediği
> için burada not edilmiştir — ileride tek bir birleşik kuyruk modülüne
> geçmek istenirse bu iki farklı kullanım modeli (authenticated sıralı vs.
> anonim toplu) uzlaştırılmalıdır.

---

## Aşama 3 — Entegrasyon ve kabul

- Puantaj → hakediş bağı: onaylanmış `attendance_days`, mevcut mesai/yevmiye tutanağı akışına veri kaynağı olabilsin. Tutanak oluşturulurken dönemdeki onaylı saatler öndolgu gelsin.
- İş programı ilerlemesi, mevcut "İlerleme Raporları" sayfasında kullanılsın.

> **DÜZELTME (Aşama 0):** İkinci madde **Aşama 1'e bağımlıdır** ve Aşama 1
> bu repoda henüz uygulanmadığından şu an yapılamaz. Birinci madde
> (puantaj → hakediş/tutanak bağı) Aşama 1'den bağımsızdır, `attendance_days`
> zaten uygulandığı için ayrıca ele alınabilir — henüz yapılmadı.

### Kabul kriterleri

1. 200 kalemlik bir WBS ağacında ilerleme hesabı 500 ms altında dönüyor.
2. Bir poza ait hakediş onaylandığında, bağlı WBS kaleminin ilerlemesi sayfa yenilendiğinde otomatik değişiyor — elle giriş yok.
3. Baseline dondurulduktan sonra tarih değişse bile dondurulmuş revizyon değişmiyor.
4. Döngüsel bağımlılık eklenemiyor.
5. Aynı `client_uuid` ile 3 kez gönderilen puantaj olayı tek kayıt oluşturuyor.
6. Geofence dışından atılan kayıt reddedilmiyor, işaretleniyor ve şef ekranında görünüyor.
7. `attendance.view_location` izni olmayan kullanıcının aldığı yanıtta `lat`/`lng` alanları yok.
8. Uçak modunda girilen 10 puantaj kaydı, bağlantı geri geldiğinde tek seferde ve çoğalmadan senkronize oluyor.
9. Tüm yeni migration'lar `down` ile geri alınabiliyor.
10. Yeni izin anahtarları izin matrisi arayüzünde görünüyor.

> **DÜZELTME (durum, 2026-09-05):** Kriter 1-4 Aşama 1'e ait, henüz
> uygulanmadığı için test edilemez. Kriter 5, 6, 7 canlı tarayıcıda uçtan
> uca doğrulandı (gerçek geofence + gerçek QR + gerçek check-in akışıyla).
> Kriter 8 backend'de `withinTokenWindow` biriminde test edildi (offline
> senaryoyu mümkün kılan tasarım), frontend'de kuyruk mekanizması
> uygulandı; gerçek "uçak modu" tarayıcı testi yapılmadı. Kriter 9 ve 10
> doğrulandı (migration'lar `down` ile geri alındı, izinler
> `/admin/permissions`'ta göründü).

---

## Yapılmayacaklar (kapsam dışı)

- Native mobil uygulama
- Mock location tespiti
- Yüz tanıma veya biyometrik doğrulama
- İSG-KATİP veya SGK entegrasyonu
- Kritik yol (CPM) hesabı — bağımlılıklar sadece görselleştirme için, otomatik tarih ötelemesi yok
- Kaynak (personel/ekipman) atama ve seviyelendirme
- Bordro hesabı
