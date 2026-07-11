# Faz 3 API — Taşeron ve Hakediş (Finansal Çekirdek)

Tüm uçlar kimlik doğrulamalıdır ve proje kapsamlıdır (`projectID` URL'den çözülür;
`X-Project-Id` başlığı da desteklenir). Yanıt zarfı, hata kodları ve optimistic
locking (`row_version` → 409 `CONFLICT` + `current_version`) Faz 1/2 ile aynıdır.

İki çapraz kesme kuralı tüm bu modülde geçerlidir:
- **Satır seviyesi güvenlik**: `project_members.subcontractor_id` ile bir taşerona
  bağlı kullanıcı (SubcontractorRep) yalnızca kendi taşeronunun kayıtlarını görür;
  aksi 403. PM/Admin/SiteEngineer kısıtsızdır.
- **view_financials**: `progress_payments.view` metrajı gösterir; birim fiyat/tutar
  alanları yalnızca `progress_payments.view_financials` ile döner (aksi halde `null`).

## Taşeronlar

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/subcontractors` | `contracts.view` | Satır-seviyesi filtreli liste |
| POST | `/api/v1/projects/{projectID}/subcontractors` | `contracts.upload` | `{company_name, tax_no?, contact_person?, phone?, email?, trade?}` |
| GET | `/api/v1/projects/{projectID}/subcontractors/{id}` | `contracts.view` | |
| PATCH | `/api/v1/projects/{projectID}/subcontractors/{id}` | `contracts.upload` | `row_version?` |
| DELETE | `/api/v1/projects/{projectID}/subcontractors/{id}` | `contracts.delete` | Soft delete (bağlı kayıt varsa 409) |

## Birim fiyat cetveli (work_items / BOQ)

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/subcontractors/{subID}/work-items` | `contracts.view` | `unit_price`/`contract_amount` yalnızca view_financials |
| POST | `.../work-items` | `contracts.upload` | `{poz_no, description, unit?, contract_qty?, unit_price?}` |
| POST | `.../work-items/import` | `contracts.upload` | **multipart**: `file` (.xlsx/.csv). Sütun: poz_no, açıklama, birim, miktar, b.fiyat. Upsert `(taşeron, poz_no)` |
| PATCH | `.../work-items/{id}` | `contracts.upload` | `row_version?` |
| DELETE | `.../work-items/{id}` | `contracts.delete` | Hakediş kalemi varsa 409 |

## Sözleşmeler (alt sözleşme arşivi)

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/contracts` | `contracts.view` | `?subcontractor_id=` filtresi; `amount`/`advance_amount` yalnızca view_financials |
| POST | `/api/v1/projects/{projectID}/contracts` | `contracts.upload` | `{contract_no, type?, subcontractor_id?, amount?, advance_amount?, retention_pct?, advance_rate_pct?, sign_date?, document_id?}` |
| PATCH | `/api/v1/projects/{projectID}/contracts/{id}` | `contracts.upload` | `row_version?` |
| DELETE | `/api/v1/projects/{projectID}/contracts/{id}` | `contracts.delete` | Soft delete |

`type`: `Main | Sub | Addendum`. `document_id` Faz 2 doküman motoruna bağdır
(sözleşme PDF'i versiyonlu arşivde tutulur). Avans/teminat/avans-mahsup oranları
hakediş hesabını sürer.

## Hakedişler (kümülatif iş akışı)

| Metot | Yol | İzin | Notlar |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/payments` | `progress_payments.view` | `?subcontractor_id=` filtresi |
| POST | `/api/v1/projects/{projectID}/payments` | `progress_payments.create_draft` | `{subcontractor_id, period_no?, period_start?, period_end?, vat_pct?}`; `period_no` verilmezse otomatik artar |
| GET | `/api/v1/projects/{projectID}/payments/{id}` | `progress_payments.view` | Hakediş + kalemler + kesintiler |
| PATCH | `/api/v1/projects/{projectID}/payments/{id}` | `progress_payments.edit_draft` | Metraj girişi + ekstra kesinti → yeniden hesap. **Yalnızca Draft**. `{items:[{work_item_id,cum_qty}], deductions:[{type,description,amount}], period_start?, period_end?, vat_pct?, row_version?}` |
| POST | `.../payments/{id}/submit` | `progress_payments.submit` | Draft → Submitted |
| POST | `.../payments/{id}/approve` | `progress_payments.approve` | Submitted → SiteApproved |
| POST | `.../payments/{id}/reject` | `progress_payments.approve` | Submitted/SiteApproved → Rejected |
| POST | `.../payments/{id}/finalize` | `progress_payments.finalize` | SiteApproved → Finalized (yeniden hesap + **DB kilidi**) |
| GET | `.../payments/{id}/summary.pdf` | `progress_payments.view_financials` | Hakediş özet PDF'i (`application/pdf`) |

### İş akışı ve hesap

Durumlar: `Draft → Submitted → SiteApproved → Finalized` (+ `Rejected`).
Geçişler hem uygulama (`CanTransition`) hem DB tarafında zorlanır. `Finalized`
kayıt ve kalemleri trigger ile UPDATE/DELETE'e kapanır → kilit ihlali 409 döner.

Hesap (Plan §6.4): her poz için `bu_dönem = cum_qty − prev_cum_qty` (önceki
`Finalized` hakedişten taşınır); brüt kümülatif A, önceki dönem B, bu dönem C.
Kesintiler: D avans mahsubu (kalan avansı aşamaz), E teminat, F/G/H ekstra
(İSG/vergi/diğer). Net (KDV hariç) I = C − ΣD..H; KDV ayrı satır. Kesinleştirmede
kayıtlı metraj + güncel birim fiyat + kayıtlı manuel kesintilerle yeniden hesaplanır.
