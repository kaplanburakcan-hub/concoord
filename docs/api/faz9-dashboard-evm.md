# Faz 9 — Dashboard, EVM ve Yönetim Raporlaması API Sözleşmesi

Temel yol: `/api/v1`. Tüm uçlar kimlik doğrulamalı.

Rol duyarlılık backend'de uygulanır:
- **EVM/tutar bloğu** yalnızca `reports.view_financial_reports` izni olana
  serileştirilir (dashboard yanıtında `evm` alanı yoksa izin yoktur).
- **Taşeron temsilcisi** (`project_members.subcontractor_id` dolu) bekleyen
  hakediş ve açık bulgu sayaçlarını yalnızca kendi firması için görür;
  aktivite akışı ve portföy ona dönmez (`subcontractor_scoped: true`).

## Dashboard

### GET /projects/{projectID}/dashboard
`projects.view`. Rol duyarlı tek DTO:

```json
{ "dashboard": {
  "project": { "id":"…","code":"PRJ-01","name":"…","status":"Active","currency":"TRY" },
  "progress_pct": 25.0,
  "milestones": [ { "id":"…","name":"Temel","planned_date":"2026-01-31",
    "actual_date":null,"weight_pct":10,"status":"InProgress","late":true } ],
  "evm": { "bac":1000000,"pv":300000,"ev":250000,"ac":235000,
    "spi":0.833,"cpi":1.064,"eac":940000,"etc":705000,
    "progress_pct":25.0,"plan_source":"manual","as_of_month":"2026-02",
    "s_curve":[ { "month":"2026-01","pv":100000,"ev":100000,"ac":105000 } ] },
  "open_findings": { "total":4,"critical":1,"major":1,"minor":2,"observation":0,"overdue":1 },
  "pending": { "payments":2,"mars":1,"prs":0,"overdue_pos":1,"open_tasks":7 },
  "activity": [ { "entity":"progress_payments","entity_id":"…",
    "from_status":"Submitted","to_status":"SiteApproved","actor":"…","at":"…" } ],
  "subcontractor_scoped": false } }
```

Notlar: `spi`/`cpi` **0 = tanımsız** (payda 0). `plan_source`:
`manual` (aylık dağılım girişi) | `milestones` (ağırlıklardan) | `linear`.
EVM tanımları için bkz. ADR-0010/2.

### GET /portfolio
`projects.view`. Kullanıcının görebildiği (üyelik veya global GRANT),
arşivlenmemiş projeler; taşeron kapsamlı olduğu projeler atlanır.
`spi`/`cpi`/`net_payable_cum` yalnızca ilgili projede finansal izin varsa döner.

```json
{ "portfolio": [ { "project_id":"…","code":"PRJ-01","name":"…","status":"Active",
  "currency":"TRY","progress_pct":25.0,"spi":0.833,"cpi":1.064,
  "open_findings":4,"pending_approvals":3,"net_payable_cum":235000 } ] }
```

## PV Aylık Dağılım Girişi

### GET /projects/{projectID}/pv-plan — `reports.view_financial_reports`
### PUT /projects/{projectID}/pv-plan — `projects.edit`
Tam liste gelir (replace-all, tek transaction). Aylar `YYYY-MM`, tekil;
`planned_pct` dönemsel yüzde ve toplam **100 ± 0.5** olmalı (aksi 422).
Boş liste = manuel plan silinir → S-eğrisi milestone ağırlıklarına
(o da yoksa doğrusala) düşer.

```json
{ "entries": [ { "month":"2026-01","planned_pct":10 },
               { "month":"2026-02","planned_pct":20 } ] }
```

## Kontrol Eşikleri (otomatik uyarı parametreleri)

### GET /projects/{projectID}/control-settings — `projects.view`
### PUT /projects/{projectID}/control-settings — `projects.edit`
Satır yoksa varsayılanlar döner: `{ "cpi_min":0.90,"spi_min":0.90,"finding_aging_days":14 }`.
Doğrulama: CPI/SPI (0,2] aralığında, yaşlanma ≥ 1 gün.

Worker 6 saatte bir tarar: CPI/SPI eşik ihlali, planlanan tarihi geçmiş
tamamlanmamış milestone, eşikten uzun süredir açık İSG bulgusu → PY
üyelerine bildirim (`evm_threshold_alert`, `milestone_late_alert`,
`finding_aging_alert`). Aynı uyarı aynı ay içinde **bir kez** bildirilir
(`control_alerts` tekilleştirme defteri).

## Aylık Yönetim Raporu

İzin: tüm uçlar `reports.view_financial_reports`; üretim ek olarak
`reports.generate_weekly` ister.

### POST /projects/{projectID}/monthly-reports
Gövde: `{ "month": "2026-02" }`. Snapshot (EVM + ay içinde kesinleşen
hakedişler ve kesinti dökümü + milestone gerçekleşme + İSG performansı +
tedarik özeti) SENKRON derlenip dondurulur, kayıt `Pending` açılır ve
`monthly_report_pdf` işi kuyruğa atılır → **202**
`{ "id":"…","status":"Pending","year":2026,"month":2 }`.
EVM kümülatifleri rapor ayının sonuna göre kesilir (geriye dönük üretim).

### GET /projects/{projectID}/monthly-reports
Son 100 kayıt: dönem, durum (`Pending|Ready|Failed`), üreten, `has_pdf`.

### GET /projects/{projectID}/monthly-reports/{id}
Üst veri + **snapshot aynen** (PDF'teki her rakam buradan doğrulanır).

### GET /projects/{projectID}/monthly-reports/{id}/download
`Ready` PDF'i `application/pdf` olarak akıtır; değilse **404**.

## Bildirim türleri (Faz 9)
`monthly_report_ready` · `monthly_report_failed` · `evm_threshold_alert` ·
`milestone_late_alert` · `finding_aging_alert`
