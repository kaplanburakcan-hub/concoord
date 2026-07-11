# ADR-0008 — Faz 7: Satınalma ve Tedarik Zinciri Kararları

Tarih: 07.07.2026 · Durum: Kabul edildi

## 1. PR yaşam döngüsü kilidi DB'de, tek beyaz listeli geçişle

**Karar:** `purchase_requests` üzerinde trigger kilidi: **Draft dışındaki
kalemler değiştirilemez**; karara bağlanmış (`Approved/Rejected/Converted`)
başlıkta yalnızca `Approved → Converted` geçişine izin verilir. Yalnızca
taslak silinebilir (soft delete). Ret sonrası düzeltme = yeni PR (revizyon
zinciri hakedişteki gibi ağır bir yapı gerektirmedi; PR ucuz ve sık üretilen
bir kayıttır).

**Gerekçe:** Plan §5.1 "hiçbir değişiklik izsiz kalmaz". Onaylanmış talep,
sipariş tutarının dayanağıdır; sonradan kalem değişirse PR→PO izlenebilirliği
anlamını yitirir. Kilit veritabanında — hakediş `Finalized` ve günlük rapor
`Submitted` desenleriyle tutarlı (SQLSTATE 23001 → API 409).

## 2. PR→PO dönüşümü tek transaction, bağ `po.pr_id` ile

**Karar:** `POST /purchase-requests/{id}/convert` aynı tx içinde PR'ı
`Converted` yapar ve `pr_id` referanslı PO açar; iki `workflow_transitions`
kaydı (PR geçişi + PO açılışı) yazılır. PR'sız acil alım için bağımsız
`POST /purchase-orders` ayrıca vardır (`pr_id NULL`).

**Gerekçe:** Kabul kriteri "PR→PO→teslimat zinciri uçtan uca izlenebiliyor".
Dönüşüm iki ayrı istekte yapılsaydı yarım kalan durum (Converted ama PO'suz)
oluşabilirdi; tek tx bunu yapısal olarak engeller.

## 3. PO statüsü teslimat kayıtlarından türetilir

**Karar:** Statü elle set edilmez; `POST /deliveries` çağrısı
`mark_delivered=false` ise `PartiallyDelivered`, `true` ise `Delivered`
üretir. `Cancelled` yalnızca açık siparişte ayrı uçla mümkündür. Kapanmış
(Delivered/Cancelled) siparişe teslimat eklenemez ve künye düzenlenemez.

**Gerekçe:** Kısmi teslimat gereksinimi (Plan Faz 7) doğal olarak
olay-güdümlüdür: durum, teslimat kanıtlarının toplamıdır. Elle statü,
irsaliyesiz "teslim edildi" kaydına kapı açardı.

## 4. İrsaliye fotoğrafı Faz 2 doküman motorunda, `deliveries.document_id` bağıyla

**Karar:** Fotoğraf ayrı bir upload yolu almaz: arayüz önce
`doc_category='Delivery'` dokümanı oluşturup versiyon yükler (mobilde
`capture="environment"` ile doğrudan kamera), sonra teslimatı `document_id`
ile kaydeder (Plan §6.6 şeması birebir).

**Gerekçe:** Versiyonlama, SHA-256 ve presigned indirme hazır gelir; ikinci
bir dosya altyapısı mimari borç olurdu (Plan §9).

## 5. Gecikme uyarısı: pano senkron, bildirim worker'da

**Karar:** İki katman: (a) `GET /procurement/board` geciken PR'ları
(ihtiyaç tarihi geçmiş, `Submitted/Approved`) ve geciken PO'ları (beklenen
tarih geçmiş, `Ordered/PartiallyDelivered`) her istekte canlı hesaplar;
(b) worker saatlik taramayla geciken PO için siparişi açana + PR sahibine
`po_overdue` bildirimi üretir. Tekrar bildirim `overdue_notified_at` ile
engellenir; beklenen tarih güncellenirse alan sıfırlanır ve uyarı yeniden
kurulur (görev deadline deseniyle aynı).

**Gerekçe:** Kabul kriteri "ihtiyaç tarihi geçmiş, teslim edilmemiş kalemler
uyarı üretiyor". Pano anlık doğruluk verir; bildirim, panoya bakmayanı da
yakalar. Saatlik tarama yeterli — gecikme günlük çözünürlüklü bir olaydır.

## 6. Satır seviyesi ek filtre yok

**Karar:** Satınalma kayıtları için taşeron/Client kapsam filtresi
uygulanmaz; erişim yalnızca `procurement.*` izinleriyle yönetilir
(SubcontractorRep ve Client rol varsayılanında salt `view` vardır).

**Gerekçe:** Plan §4 satır güvenliğini hakediş/MAR için tanımlar; satınalma
şirket içi bir süreçtir. İleride tedarikçi portalı gerekirse override +
kapsam filtresi eklenebilir (izin sözlüğü veri olduğu için kod değişikliği
gerektirmez).
