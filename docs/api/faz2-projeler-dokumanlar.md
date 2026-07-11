# İPKS API — Faz 2: Proje Çekirdeği ve Doküman Motoru

Tüm yanıtlar JSON; hatalar Faz 0 error envelope'unu kullanır. Korumalı uçlar
`Authorization: Bearer <access_token>` bekler. Faz 2 uçlarının tamamı proje
kapsamlıdır ve proje `{projectID}` URL segmentinden çözülür; izin kontrolü
`(kullanıcı, proje, izin)` üçlüsüyle yapılır (DENY > GRANT > rol).

## Projeler

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects` | oturum | Yalnızca üyesi olunan projeler; global `projects.view` GRANT'i olan (bootstrap admin) tümünü görür. Proje seçicinin kaynağı. |
| POST | `/api/v1/projects` | `projects.create` | `{code, name, location?, client_name?, budget_total?, currency?, start_date?, end_date?, status?}`. Oluşturan otomatik **ProjectManager** üye olur. |
| GET | `/api/v1/projects/{projectID}` | `projects.view` | Tekil künye |
| PATCH | `/api/v1/projects/{projectID}` | `projects.edit` | Künye alanları + `row_version` (optimistic lock) |
| DELETE | `/api/v1/projects/{projectID}` | `projects.delete` | Soft delete → statü `Archived` |

`status`: `Planning | Active | OnHold | Closed | Archived`. `code` yaşayan
kayıtlarda benzersiz (çakışma → 409).

## Milestone'lar

| Metot | Yol | İzin |
|---|---|---|
| GET | `/api/v1/projects/{projectID}/milestones` | `projects.view` |
| POST | `/api/v1/projects/{projectID}/milestones` | `projects.edit` |
| PATCH | `/api/v1/projects/{projectID}/milestones/{id}` | `projects.edit` |
| DELETE | `/api/v1/projects/{projectID}/milestones/{id}` | `projects.edit` |

Gövde: `{name, planned_date?, actual_date?, weight_pct?, status?, sort_order?, row_version?}`.
`status`: `Planned | InProgress | Completed | Delayed`. `weight_pct` 0–100.
Tarihler `YYYY-MM-DD` (boş/null geçilebilir).

## Klasörler (doküman ağacı)

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/folders` | `documents.view` | Düz liste; istemci `parent_id` ile ağaç kurar |
| POST | `/api/v1/projects/{projectID}/folders` | `documents.manage_folders` | `{name, parent_id?, module_scope?}` |
| PATCH | `/api/v1/projects/{projectID}/folders/{id}` | `documents.manage_folders` | Yeniden adlandır/taşı (döngü koruması) |
| DELETE | `/api/v1/projects/{projectID}/folders/{id}` | `documents.manage_folders` | Yalnızca **boş** klasör silinir (409 aksi halde) |

Aynı üst klasör altında isim tekildir (çakışma → 409).

## Dokümanlar ve versiyonlar

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/documents` | `documents.view` | Filtre: `?folder_id=&entity_type=&entity_id=&category=`; her satır versiyon sayısı + en son versiyonu taşır |
| POST | `/api/v1/projects/{projectID}/documents` | `documents.upload` | `{title, folder_id?, doc_category?, entity_type?, entity_id?}` (metadata; dosya ayrı yüklenir) |
| GET | `/api/v1/projects/{projectID}/documents/{id}` | `documents.view` | Doküman + tüm versiyonlar (SHA-256, boyut, yükleyen) |
| PATCH | `/api/v1/projects/{projectID}/documents/{id}` | `documents.upload` | `{title?, folder_id?, doc_category?, row_version?}` |
| DELETE | `/api/v1/projects/{projectID}/documents/{id}` | `documents.delete` | Soft delete |
| POST | `/api/v1/projects/{projectID}/documents/{id}/versions` | `documents.upload` | **multipart/form-data**: `file` (zorunlu), `note?`. Sunucu SHA-256+boyut hesaplar, MinIO'ya yazar, `v{n}` üretir |
| GET | `/api/v1/projects/{projectID}/documents/{id}/versions/{versionNo}/download` | `documents.download` | Nesneyi akışla indirir (`Content-Disposition: attachment`) |

`doc_category`: `Contract | Addendum | Submittal | Drawing | Delivery | OHS | Other`.
Polimorfik bağ (`entity_type`/`entity_id`) aynı motorun Faz 3+ modüllerine
(hakediş, İSG tutanağı, irsaliye) hizmet etmesini sağlar; Faz 2 ilk tüketici
sözleşme/zeyilname arşividir.

### Güvenlik
- Yükleme/indirme **daima API üzerinden** geçer; her istek izin + proje
  kapsamı kontrolünden geçer. Depolama anahtarı ve MinIO uç noktası istemciye
  **asla** dönmez → "URL'i bilse dahi yetkisiz erişemez".
- Yükleme sınırı 100 MB. Nesne, DB kaydından **önce** yazılır (başarısızsa yetim
  kayıt oluşmaz); versiyon benzersizliği DB kısıtıyla garanti edilir.
- Her versiyon immutable'dır; düzeltme yeni versiyon (`v2, v3, …`) olarak eklenir,
  eski versiyon daima indirilebilir kalır (Plan §5.2).
