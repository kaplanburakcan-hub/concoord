# İPKS API — Faz 5: Malzeme Onay Süreci (MAR)

Tüm uçlar `Authorization: Bearer <access>` ister. Proje kapsamlı izinler
`RequirePermission` ile denetlenir; satır seviyesi güvenlik (taşeron kapsamı)
ve Client kısıtlı görünüm handler içinde ZORUNLU uygulanır.

## Statü akışı
```
Submitted ──review──► UnderReview ──decide──► Approved
                                          ├─► ConditionallyApproved
                                          └─► Rejected
```
- Client, `Submitted` kayıtları göremez (liste/tekil/CSV'de aynı filtre).
- SubcontractorRep yalnızca kendi firmasının MAR'larını görür ve yalnızca
  kendi firması adına MAR açabilir.
- Karar notu (`decision_note`) karar isteğinde ZORUNLUDUR.

## Uçlar

### `GET /api/v1/projects/{projectID}/materials` — izin: `material_approvals.view`
Sorgu: `?status=Submitted|UnderReview|Approved|ConditionallyApproved|Rejected` (ops.)
```json
{ "material_approvals": [ {
  "id": "…", "mar_no": "MAR-001", "material_name": "C30/37 Hazır Beton",
  "spec_ref": "TS EN 206", "manufacturer": "…",
  "subcontractor_id": "…", "subcontractor_name": "…",
  "status": "UnderReview",
  "decision_note": null, "decided_by": null, "decided_by_name": null, "decided_at": null,
  "created_by": "…", "created_by_name": "…",
  "attachment_count": 2, "row_version": 3, "created_at": "…"
} ] }
```

### `POST /api/v1/projects/{projectID}/materials` — izin: `material_approvals.create`
```json
{ "material_name": "C30/37 Hazır Beton", "spec_ref": "TS EN 206",
  "manufacturer": "…", "subcontractor_id": "… (ops.)" }
```
`201` → MAR nesnesi. `mar_no` sunucu tarafından proje içinde sıralı üretilir.
SubcontractorRep için `subcontractor_id` yok sayılır ve kendi firması zorlanır.

### `GET /api/v1/projects/{projectID}/materials/{id}` — izin: `material_approvals.view`
Kapsam dışı kayıt `404` döner (bilgi sızıntısı yok).

### `PATCH /api/v1/projects/{projectID}/materials/{id}` — izin: `material_approvals.create`
Künye düzenleme; yalnızca `Submitted` durumunda (`409` aksi halde).

### `POST /api/v1/projects/{projectID}/materials/{id}/review` — izin: `material_approvals.review`
`Submitted → UnderReview`. Bu geçiş MAR'ı müşavire/işverene sunar; karar
yetkililerine `mar_under_review` bildirimi düşer.

### `POST /api/v1/projects/{projectID}/materials/{id}/decide` — izin: `material_approvals.decide`
```json
{ "decision": "Approved | ConditionallyApproved | Rejected",
  "decision_note": "zorunlu — gerekçe/şartlar" }
```
Yalnızca `UnderReview` durumunda (`409` aksi halde). `decision_note` boşsa
`422 validation_error`. Karar sonrası talep sahibi + taşeron temsilcilerine
`mar_decided` bildirimi düşer.

### `GET /api/v1/projects/{projectID}/materials/register.csv` — izin: `material_approvals.view`
MAR kayıt defteri; `;` ayırıcılı, UTF-8 BOM'lu CSV (TR Excel uyumlu). İçerik,
isteği yapan kullanıcının kapsam filtreleriyle sınırlıdır.

## Doküman ekleri (Faz 2 motoru)
MAR eki = polimorfik bağlı doküman:
- Listeleme: `GET /projects/{pid}/documents?entity_type=material_approval&entity_id={marID}`
- Oluşturma: `POST /projects/{pid}/documents` gövde
  `{ "title": "…", "doc_category": "Submittal", "entity_type": "material_approval", "entity_id": "{marID}" }`
- Dosya: `POST /projects/{pid}/documents/{docID}/versions` (multipart)

## Bildirim türleri (Faz 4 motoru)
`mar_submitted` · `mar_under_review` · `mar_decided`
