# Faz 8 — İSG Modülü API Sözleşmesi

Temel yol: `/api/v1`. Tüm uçlar kimlik doğrulamalı; izinler: `ohs.view`,
`ohs.perform_inspection`, `ohs.issue_penalty`, `ohs.manage_checklists`.
Tutar görünürlüğü hakedişle aynı ayrıma tabidir
(`progress_payments.view_financials`); taşeron temsilcisi kendi firmasının
tutarını her koşulda görür (sözleşme tarafıdır).

Satır seviyesi güvenlik: taşeron temsilcisi yalnızca **kendi firmasının**
bulgu ve ceza kayıtlarını görür — filtre backend'de zorunludur.

Değişmezlik: gönderilmiş denetim ve kesilmiş ceza tutanağı DB trigger'ıyla
kilitlidir (değişiklik denemesi → **409**); her statü geçişi
`workflow_transitions`a yazılır.

## Checklist Şablonları (global)

### GET /ohs/checklist-templates?active=true
`ohs.view`. `active=true` yalnız aktifleri döner (denetim ekranı).

```json
{ "templates": [ { "id": "…", "name": "Yüksekte Çalışma", "category": "Genel",
  "items": [ { "no": 1, "text": "Baret kullanılıyor", "critical": true } ],
  "is_active": true, "row_version": 1, "created_at": "…" } ] }
```

### POST /ohs/checklist-templates · PATCH /ohs/checklist-templates/{id} · DELETE …/{id}
`ohs.manage_checklists`. Maddeler: benzersiz pozitif `no`, boş olmayan `text`.
Şablon güncellemesi geçmiş denetimleri ETKİLEMEZ (denetim yanıtları kendi
JSONB'sinde taşınır). Silme = soft delete.

## Denetimler

### GET /projects/{projectID}/ohs/inspections
`ohs.view`. En yeni 500 kayıt; `score` (uygunluk %) ve `fail_count` içerir.

### POST /projects/{projectID}/ohs/inspections
`ohs.perform_inspection`. Denetim tek adımda oluşur (Submitted) ve KİLİTLENİR.
Offline kuyruk bu ucu kullanır; `inspected_at` cihaz saati olarak gönderilir.

```json
{
  "template_id": "…",
  "inspected_at": "2026-07-08T09:30:00Z",
  "location_text": "B blok 3. kat",
  "gps_lat": 39.92, "gps_lng": 32.85,
  "results": [
    { "no": 1, "answer": "ok" },
    { "no": 2, "answer": "fail", "note": "İki işçi baretsiz" },
    { "no": 3, "answer": "na" }
  ]
}
```

Doğrulama: her şablon maddesine tam olarak bir yanıt (`ok|fail|na`).
Skor sunucuda hesaplanır: `ok / (ok+fail)`; `na` payda dışı. Yanıt:
`201 { "id": "…", "score": 66.67 }`.

### GET /projects/{projectID}/ohs/inspections/{id}
`ohs.view`. `results` dahil tam kayıt.

## Bulgular

### GET /projects/{projectID}/ohs/findings?status=
`ohs.view`. `overdue=true`: termini geçmiş ve kapanmamış. `age_days`:
yaşlandırma (haftalık rapor Faz 9'da tüketir).

### POST /projects/{projectID}/ohs/findings
`ohs.perform_inspection` (taşeron hesabı bulgu açamaz → 403). Fotoğraf,
offline kuyruğun JSON-only kısıtına uyum için **data-URL** olarak gövdeye
gömülür (≤ 8 MB, yalnız `image/*`); sunucu tek istekte Faz 2 doküman motoruna
(kategori `OHS`, `entity_type=ohs_finding`) yazar.

```json
{
  "severity": "Major",
  "description": "Korkuluksuz döşeme kenarı",
  "subcontractor_id": "…",
  "location": "B blok 3. kat",
  "gps_lat": 39.92, "gps_lng": 32.85,
  "due_date": "2026-07-12",
  "photo_base64": "data:image/jpeg;base64,…",
  "photo_name": "IMG_0042.jpg"
}
```

### POST …/findings/{id}/start · POST …/findings/{id}/close
Yaşam döngüsü `Open → InProgress → Closed`. `close` için not ZORUNLU;
kapatma yalnızca saha tarafı (`ohs.perform_inspection`). Taşeron temsilcisi
kendi firmasının bulgusunu yalnızca `start` ile ilerletebilir. Kapanan bulgu
DB kilidiyle değiştirilemez. Gövde: `{ "note": "…", "row_version": 3 }`.

Termin taraması: worker saatte bir süresi geçen açık bulgular için
`ohs_finding_overdue` bildirimi üretir (bulgu başına bir kez).

## Ceza Tutanakları

### POST /projects/{projectID}/ohs/penalties
`ohs.issue_penalty`. **Otomasyon tek istekte, senkron:** tutanak kaydı +
PDF üretimi (MinIO + `files`) + workflow/audit + bildirimler (taşeron
temsilcileri `ohs_penalty_issued` + hakediş kesinleştirme yetkisi olanlar).
Kabul kriteri (≤ 60 sn) yapısal olarak sağlanır — PDF stdlib motorla
milisaniyelerde üretilir.

```json
{
  "subcontractor_id": "…",
  "finding_id": "…",
  "violation_type": "Baretsiz çalışma",
  "penalty_level": "Fine",
  "amount": 2500,
  "note": "İkinci tekrar.",
  "evidence_base64": "data:image/jpeg;base64,…"
}
```

Kurallar: `Fine` → `amount` zorunlu ve > 0; `Warning` → tutar girilemez.
Tutanak no proje içinde sıralı üretilir (`ISG-001`, advisory lock).
Yanıt: `201 { "id": "…", "penalty_no": "ISG-003", "pdf_ready": true }`.

### GET …/penalties · GET …/penalties/{id} · GET …/penalties/{id}/pdf
`ohs.view`. PDF kimlikli akışla iner (depolama anahtarı sızmaz).

### POST …/penalties/{id}/acknowledge
Yalnızca ilgili taşeron temsilcisi; `Issued → Acknowledged` (tebellüğ).

## Hakediş Köprüsü (Faz 3 entegrasyonu)

Taslak hakediş detayı (`GET /projects/{pid}/payments/{id}`,
`view_financials` ile) artık şunu içerir:

```json
"ohs_penalty_suggestions": [
  { "penalty_id": "…", "penalty_no": "ISG-003", "violation_type": "Baretsiz çalışma",
    "issued_at": "2026-07-07", "amount": 2500, "already_added": false } ]
```

Öneri kümesi: taşerona kesilmiş, `Fine`, statüsü `Issued|Acknowledged`
(yani henüz hiçbir hakedişe uygulanmamış) tutanaklar. PY öneriyi taslağa
kesinti olarak ekler (`PATCH` gövdesindeki `deductions[]` artık
`source_entity: "ohs_penalties"`, `source_id` alanlarını taşır ve korur).
Hakediş **Finalized** olduğunda bağlı tutanaklar aynı transaction içinde
`AppliedToPayment` + `applied_payment_id` olarak işaretlenir — çift kesinti
yapısal olarak engellenir, izlenebilirlik `source_id` üzerindendir.
