# Faz 7 — Satınalma ve Tedarik Zinciri API Sözleşmesi

Temel yol: `/api/v1`. Tüm uçlar kimlik doğrulamalı; proje kapsamlı izinler:
`procurement.view`, `procurement.create_pr`, `procurement.approve_pr`,
`procurement.manage_po`, `procurement.upload_delivery`.

Zincir: **PR** `Draft → Submitted → Approved | Rejected`; `Approved →
Converted` (PO oluşur). **PO** `Ordered → PartiallyDelivered → Delivered`
veya `→ Cancelled`. Kesinleşmiş kayıtlar DB trigger'ıyla kilitlidir
(değişiklik denemesi → **409**); her geçiş `workflow_transitions`a yazılır.

## Satınalma Talepleri (PR)

### GET /projects/{projectID}/purchase-requests?status=
`procurement.view`. En yeni 500 kayıt. `overdue=true` alanı: ihtiyaç tarihi
geçmiş ve henüz siparişe dönmemiş (`Submitted/Approved`) talepler.

```json
{ "purchase_requests": [ { "id": "…", "pr_no": "PR-001", "status": "Submitted",
  "needed_by_date": "2026-07-15", "requested_by_name": "…", "overdue": false,
  "item_count": 3, "po_no": null, "row_version": 2, "created_at": "…" } ] }
```

### POST /projects/{projectID}/purchase-requests
`procurement.create_pr`. Taslak PR + kalemleri. PR numarası proje içinde
sıralı üretilir (advisory lock ile yarışsız).

```json
{
  "needed_by_date": "2026-07-15",
  "note": "B blok kaba imalat",
  "items": [
    { "material_name": "C30 hazır beton", "spec": "TS EN 206", "qty": 120, "unit": "m³" },
    { "material_name": "Nervürlü demir Ø16", "qty": 8.5, "unit": "ton", "note": "S420" }
  ]
}
```
Yanıt: `201` — kalemli PR gövdesi. Doğrulama hataları alan bazlı
`400 validation_error` (en az bir kalem, miktar > 0, birim zorunlu).

### GET /projects/{projectID}/purchase-requests/{id}
`procurement.view`. Kalemli tekil PR (`items[]` dahil).

### PATCH /projects/{projectID}/purchase-requests/{id}
`procurement.create_pr`. Yalnızca `Draft` düzenlenir; kalem seti tümden
değiştirilir (eskiler soft delete + yeniler). Diğer durumlar **409**.

### DELETE /projects/{projectID}/purchase-requests/{id}
`procurement.create_pr`. Yalnızca taslak silinir (soft). Aksi **409**.

### POST /projects/{projectID}/purchase-requests/{id}/submit
`procurement.create_pr`. `Draft → Submitted`; kalemsiz PR gönderilemez.
Onay yetkililerine `pr_submitted` bildirimi gider.

### POST /projects/{projectID}/purchase-requests/{id}/approve
### POST /projects/{projectID}/purchase-requests/{id}/reject
`procurement.approve_pr`. `Submitted → Approved | Rejected`. Gövde:
`{ "decision_note": "…" }` — **ret için zorunlu**. Talep sahibine
`pr_decided` bildirimi gider.

### POST /projects/{projectID}/purchase-requests/{id}/convert
`procurement.manage_po`. `Approved → Converted` + aynı transaction'da
`pr_id` bağlı yeni PO. Gövde PO künyesidir (aşağıdaki POST /purchase-orders
ile aynı). Yanıt: `201` — PO gövdesi. Approved olmayan PR **409**.

## Siparişler (PO)

### GET /projects/{projectID}/purchase-orders?status=&overdue=1
`procurement.view`. `overdue=true`: beklenen tarih geçmiş, kapanmamış.

### POST /projects/{projectID}/purchase-orders
`procurement.manage_po`. Bağımsız (PR'sız) sipariş.

```json
{ "supplier_name": "Yılmaz Yapı Malz. A.Ş.", "amount": 850000,
  "currency": "TRY", "expected_date": "2026-07-25", "note": "…" }
```
`amount`/`expected_date` opsiyonel; `currency` varsayılanı `TRY`.

### GET /projects/{projectID}/purchase-orders/{id}
`procurement.view`. Teslimat zinciriyle (`deliveries[]`) tekil sipariş.

### PATCH /projects/{projectID}/purchase-orders/{id}
`procurement.manage_po`. Yalnızca açık (`Ordered/PartiallyDelivered`)
sipariş düzenlenir; `expected_date` değişirse gecikme uyarısı yeniden
kurulur. Kapanmış sipariş **409**.

### POST /projects/{projectID}/purchase-orders/{id}/cancel
`procurement.manage_po`. Açık sipariş → `Cancelled`. Aksi **409**.

### POST /projects/{projectID}/purchase-orders/{id}/deliveries
`procurement.upload_delivery`. Kısmi teslimat kaydı; statüyü otomatik
ilerletir.

```json
{ "delivery_note_no": "İRS-2026-0042",
  "delivered_at": "2026-07-07T09:30:00+03:00",
  "document_id": "…",
  "note": "40 m³ — 1. parti",
  "mark_delivered": false }
```
- `mark_delivered=false` → `PartiallyDelivered`; `true` → `Delivered` (kapatır).
- `document_id`: irsaliye fotoğrafı — arayüz önce Faz 2 doküman motoruna
  (`doc_category='Delivery'`) yükler, dönen kimliği buraya bağlar. Doküman
  aynı projeye ait olmalıdır.
- Kapanmış siparişe teslimat **409**.

## Tedarik Durum Panosu

### GET /projects/{projectID}/procurement/board
`procurement.view`. Statü sayaçları + gecikenler (kabul kriteri: geciken
kalemler uyarı üretir; ek olarak worker saatlik taramada geciken PO için
`po_overdue` bildirimi gönderir — siparişi açan + PR sahibi).

```json
{
  "pr_counts": { "Draft": 2, "Submitted": 1, "Approved": 1, "Converted": 5 },
  "po_counts": { "Ordered": 3, "PartiallyDelivered": 1, "Delivered": 4 },
  "overdue_prs": [ { "pr_no": "PR-004", "needed_by_date": "2026-07-01", "…": "…" } ],
  "overdue_pos": [ { "po_no": "PO-002", "expected_date": "2026-07-03", "…": "…" } ]
}
```

## Bildirimler (Faz 4 motoru)

| Tür | Alıcı | Tetik |
|---|---|---|
| `pr_submitted` | `procurement.approve_pr` yetkili proje üyeleri | PR onaya sunuldu |
| `pr_decided` | talep sahibi | PR onaylandı/reddedildi |
| `po_overdue` | siparişi açan + (varsa) PR sahibi | beklenen teslim tarihi geçti (worker, saatlik) |
