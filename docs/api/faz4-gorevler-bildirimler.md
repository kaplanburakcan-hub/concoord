# Faz 4 API — Görev Yönetimi + Bildirim Motoru

Tüm uçlar `Authorization: Bearer <access>` gerektirir. Hatalar standart error
envelope ile döner. Görev uçları proje kapsamlı izinlerle korunur; bildirim
uçları kişiye özeldir (izin gerektirmez, veri `user_id = oturum` filtrelidir).

## Görevler (Kanban)

| Metot | Yol | İzin | Açıklama |
|---|---|---|---|
| GET | `/api/v1/projects/{projectID}/tasks` | tasks.view | Pano kaynağı; `tasks` + `status_order` döner |
| POST | `/api/v1/projects/{projectID}/tasks` | tasks.create | Yeni görev (atama verilirse ayrıca tasks.assign aranır) |
| GET | `/api/v1/projects/{projectID}/tasks/{id}` | tasks.view | Görev + atananlar |
| PATCH | `/api/v1/projects/{projectID}/tasks/{id}` | tasks.view + edit_own/edit_all* | Başlık/açıklama/öncelik/termin/atama; optimistic locking (`row_version`) |
| POST | `/api/v1/projects/{projectID}/tasks/{id}/move` | tasks.view + edit_own/edit_all* | Sürükle-bırak: `{status, kanban_order, row_version}`; statü değişimi `workflow_transitions`a yazılır |
| DELETE | `/api/v1/projects/{projectID}/tasks/{id}` | edit_all ya da (edit_own + oluşturan) | Soft delete |
| GET | `/api/v1/projects/{projectID}/tasks/{id}/comments` | tasks.view | Yorum listesi |
| POST | `/api/v1/projects/{projectID}/tasks/{id}/comments` | tasks.view | `{body}`; `@kullanıcıadı` mention'ları proje üyeleri arasında çözümlenir |
| GET | `/api/v1/projects/{projectID}/assignable-users` | tasks.view | Atama/@mention için proje üyeleri |

\* `edit_own`: kullanıcı görevi oluşturmuş **ya da** göreve atanmış olmalı.
`edit_all` her görevi kapsar. Kontrol handler içindedir (rota izni: tasks.view).

### Örnek — görev oluşturma
```json
POST /api/v1/projects/{pid}/tasks
{
  "title": "Kalıp söküm kontrolü — B blok 3. kat",
  "description": "Söküm öncesi beton dayanım raporu eklenecek.",
  "priority": "High",
  "due_date": "2026-07-15",
  "assignee_ids": ["<user-uuid>"]
}
```
Alanlar: `status` Backlog|Todo|InProgress|Review|Done (varsayılan Backlog),
`priority` Low|Normal|High|Urgent (varsayılan Normal), `due_date` YYYY-MM-DD.

### Örnek — taşıma (sürükle-bırak)
```json
POST /api/v1/projects/{pid}/tasks/{id}/move
{ "status": "InProgress", "kanban_order": 7, "row_version": 3 }
```
409 = kayıt bu arada değişti; istemci panoyu yeniler.

## Bildirimler

| Metot | Yol | Açıklama |
|---|---|---|
| GET | `/api/v1/notifications?limit=30&unread=1` | Kendi InApp bildirimleri + `unread_count` |
| POST | `/api/v1/notifications/{id}/read` | Okundu işaretle (idempotent, 204) |
| POST | `/api/v1/notifications/read-all` | Tümünü okundu işaretle (204) |
| GET | `/api/v1/notification-preferences` | Kanal haritası: `{"InApp":true,"Email":true,"SMS":false}` |
| PUT | `/api/v1/notification-preferences` | `{"channel":"Email","enabled":false}` (204) |

Varsayılan kanallar: InApp + Email açık, SMS kapalı.

### Bildirim türleri (Faz 4)
- `task_assigned` — göreve atanma
- `task_mention` — yorumda `@kullanıcıadı` ile bahsedilme
- `task_comment` — atandığınız / oluşturduğunuz göreve yorum
- `task_deadline` — termin ≤ 24 saat ya da geçmiş (worker üretir, görev başına bir kez; termin değişirse yeniden kurulur)

## Asenkron gönderim (worker)
E-posta/SMS gönderimi API isteğini bekletmez: `job_queue` tablosuna
`send_email` / `send_sms` işi yazılır; worker `FOR UPDATE SKIP LOCKED` ile
çeker. Başarısız iş üstel geri çekilmeyle (30s → 60s → … tavan 1 saat, en çok
5 deneme) yeniden denenir; kalıcı başarısızlık `failed` + `last_error` olarak
kalır. SMTP yapılandırılmamışsa e-posta loglanır ve iş tamamlanır (dev). SMS
sağlayıcısı `IPKS_SMS_PROVIDER=netgsm` ile seçilir; boşsa log adaptörü çalışır.
