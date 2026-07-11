# Faz 6 — Günlük/Haftalık Saha Raporlama API Sözleşmesi

Temel yol: `/api/v1`. Tüm uçlar kimlik doğrulamalı; proje kapsamlı izinler
`reports.view`, `reports.create_daily`, `reports.generate_weekly` ile korunur.

## Günlük Raporlar

Yaşam döngüsü: `Draft → Submitted`. **Submitted rapor değişmezdir** (DB
trigger kilidi, SQLSTATE 23001 → API 409). Geçmiş gün düzeltmesi
`/revise` ile açılan yeni revizyondur; bir tarih için geçerli rapor en yüksek
`revision_no`'dur ve eski revizyonlar izlenebilir kalır.

### GET /projects/{projectID}/daily-reports?from=&to=
`reports.view`. Tarih başına geçerli revizyonları döner (en yeni tarih önce,
en çok 200 kayıt). `from`/`to` isteğe bağlı `YYYY-MM-DD` aralığı.

```json
{ "daily_reports": [ { "id": "...", "report_date": "2026-07-06",
  "revision_no": 1, "status": "Draft", "weather": {"condition":"Açık"},
  "temperature_min": 14, "temperature_max": 27,
  "author_name": "...", "row_version": 1, "created_at": "..." } ] }
```

### POST /projects/{projectID}/daily-reports
`reports.create_daily`. Yeni taslak. Aynı tarihte kayıt varsa **409**.

```json
{
  "report_date": "2026-07-06",
  "weather": { "condition": "Açık", "source": "manual" },
  "temperature_min": 14, "temperature_max": 27,
  "notes": "…",
  "manpower":    [ { "subcontractor_id": null, "trade": "Kalıpçı", "headcount": 8 } ],
  "equipment":   [ { "equipment_name": "Kule vinç", "count": 1, "working_hours": 9, "idle_reason": null } ],
  "work_entries":[ { "work_item_id": "…|null", "location": "B blok 3. kat",
                     "description": "Perde betonu", "qty": 42.5, "unit": "m³" } ]
}
```
Yanıt: `201 { "id": "…" }`. Doğrulama hataları alan bazlı `422/400 validation_error`.
`work_item_id` verilirse imalat girdisi Faz 3 birim fiyat cetveline (BOQ) bağlanır.

### GET /projects/{projectID}/daily-reports/{id}
`reports.view`. Başlık + tüm satırlar (`manpower`, `equipment`, `work_entries`;
satırlarda `subcontractor_name` ve `work_item_poz` çözümlenmiş gelir).

### PUT /projects/{projectID}/daily-reports/{id}
`reports.create_daily`. **Yalnızca taslak.** Başlık + satırlar tam değişim.
Submitted → `409 conflict`.

### POST /projects/{projectID}/daily-reports/{id}/submit
`reports.create_daily`. `Draft → Submitted`; geçiş `workflow_transitions`a
yazılır, kayıt kilitlenir. Zaten gönderilmişse 409.

### POST /projects/{projectID}/daily-reports/{id}/revise
`reports.create_daily`. Yalnızca Submitted rapor için: başlık ve satırları
kopyalayan yeni `Draft` revizyon (`revision_no+1`, `parent_report_id` set)
oluşturur. Yanıt: `201 { "id": "<yeni revizyon>" }`. Aynı tarihte açık
(gönderilmemiş) revizyon varsa 409.

### DELETE /projects/{projectID}/daily-reports/{id}
`reports.create_daily`. Yalnızca taslak; soft delete. Submitted → 409.

### GET /projects/{projectID}/daily-reports/weather?date=&lat=&lng=
`reports.create_daily`. **Opsiyonel** hava durumu ön doldurma;
`IPKS_WEATHER_ENABLED=true` değilse 404 döner ve istemci elle doldurur.
Varsayılan sağlayıcı Open-Meteo (anahtarsız); `IPKS_WEATHER_API_URL` ile
değiştirilebilir. Konum istemciden gelir (cihaz GPS'i).

```json
{ "weather": { "condition": "Yağmurlu", "temperature_min": 11.2,
  "temperature_max": 19.8, "wind_kph": 22.4, "precipitation_mm": 6.1,
  "source": "open-meteo" } }
```

## Haftalık Raporlar

Akış: `POST` → API haftanın verisini **senkron** derler ve `snapshot` JSONB
olarak dondurur → `weekly_report_pdf` işi kuyruğa atılır → worker PDF'i
snapshot'tan üretir, MinIO'ya yükler (`project/{pid}/reports/weekly/{id}.pdf`),
kaydı `Ready` yapar ve üreteni bilgilendirir. PDF'e giren her sayı snapshot'ta
vardır; sonradan yapılan günlük rapor revizyonları üretilmiş raporu etkilemez.

### GET /projects/{projectID}/weekly-reports
`reports.view`. Hafif liste (snapshot hariç): `status` ∈
`Pending|Ready|Failed`, `has_pdf`, `error`.

### POST /projects/{projectID}/weekly-reports
`reports.generate_weekly`. Gövde: `{ "period_start": "YYYY-MM-DD" }` —
haftanın herhangi bir günü verilebilir, ISO haftasına (Pzt–Paz) yuvarlanır.
Yanıt: `202 { "id", "status": "Pending", "week_no", "period_start", "period_end" }`.

Snapshot içeriği: gün bazında hava/personel/ekipman/imalat + hafta toplamları
(adam-gün, ekipman saati, imalat girdisi sayısı), bekleyen hakediş **statüleri**
(tutarsız — `view_financials` ayrımı gereği haftalıkta finansal tutar yer almaz),
açık görev / bu hafta terminli görev sayıları, bekleyen MAR sayısı. İSG özeti
alanı Faz 8'de dolar.

### GET /projects/{projectID}/weekly-reports/{id}
`reports.view`. Kayıt + ham `snapshot` (PDF rakamlarının doğrulanması için).

### GET /projects/{projectID}/weekly-reports/{id}/download
`reports.view`. `application/pdf` akışı; yalnızca `Ready` kayıtlarda.

## Bildirimler (Faz 4 motoru üzerinden)

| Tür | Alıcı | Tetik |
|---|---|---|
| `weekly_report_ready` | Üreten kullanıcı | Worker PDF'i tamamladığında |
| `weekly_report_failed` | Üreten kullanıcı | Snapshot çözümlenemediğinde / kalıcı hata |

## PWA Çevrimdışı Kuyruk v1 (istemci)

- `frontend/public/sw.js`: uygulama kabuğu cache'i (navigasyon network-first);
  API istekleri asla cache'lenmez.
- `frontend/src/offline/queue.ts`: günlük rapor yazma istekleri (oluştur /
  taslak güncelle / gönder) çevrimdışıyken localStorage kuyruğuna alınır ve
  bağlantı dönünce **sırayla** oynatılır. Sunucu 409/4xx dönerse öğe
  `conflict/failed` işaretlenir ve kullanıcı kararına bırakılır — veri
  sessizce atılmaz. Fotoğraflı çevrimdışı form Faz 8 (İSG) kapsamındadır.

## Yapılandırma

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `IPKS_WEATHER_ENABLED` | `false` | Hava durumu ön doldurma ucunu açar |
| `IPKS_WEATHER_API_URL` | boş (Open-Meteo) | Uyumlu alternatif sağlayıcı ucu |
