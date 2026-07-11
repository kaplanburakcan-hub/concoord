# ADR-0009 — Faz 8: İSG Modülü Kararları

Tarih: 08.07.2026 · Durum: Kabul edildi

## 1. Ceza PDF'i istek içinde SENKRON üretilir (worker'a gitmez)

**Karar:** `POST /ohs/penalties` aynı istek içinde tutanak kaydını açar,
PDF'i stdlib motorla üretir (Faz 3 hakediş özeti motorunun paket-yerel
kopyası), MinIO'ya yazar ve `pdf_file_id`'yi bağlar. Haftalık rapor (Faz 6)
worker kuyruğunu kullanmaya devam eder.

**Gerekçe:** Kabul kriteri "sahada kesilen ceza 60 saniye içinde PDF +
bildirim üretiyor". Kuyruk yolu worker'ın ayakta olmasına ve tick aralığına
bağımlıdır; senkron üretim deterministik motorla milisaniyeler sürdüğünden
sınır yapısal olarak garanti edilir. Haftalık rapor ise çok kaynaklı ve
ağır bir derlemedir — kuyrukta kalması doğru.

## 2. Foto, data-URL (base64) ile TEK JSON istekte taşınır

**Karar:** Bulgu ve ceza kanıt fotoğrafı istek gövdesinde
`data:image/...;base64,` olarak kabul edilir (≤ 8 MB, yalnız `image/*`);
sunucu tek transaction akışında Faz 2 doküman motoruna (documents +
document_versions + files + MinIO) yazar ve polimorfik bağı kurar.

**Gerekçe:** Faz 6 offline kuyruğu bilinçli olarak JSON-only kuruldu ve
"fotoğraf ekleri Faz 8 kapsamında" notu düşüldü. Alternatif (doküman oluştur →
multipart yükle → bulgu oluştur) üç bağımlı istektir ve kuyruk yeniden
oynatmasında yarım kalma riski taşır. Tek JSON istek, mevcut kuyruğu HİÇ
değiştirmeden "uçak modunda foto + form" senaryosunu karşılar. Base64 ~%33
şişme maliyeti, 8 MB sınırı ve istemci tarafı sıkıştırma önerisiyle (Plan §10)
kabul edilebilir. Doküman motoru atlanmaz — Plan §9'daki mimari borç önlemi korunur.

## 3. Denetim tek adımda oluşur ve kilitlenir (sunucuda taslak yok)

**Karar:** `ohs_inspections` yalnız `Submitted` statüsüyle doğar; içerik
alanları DB trigger'ıyla değişmez, soft delete dahi kapalıdır.

**Gerekçe:** Denetim saha KANITIDIR (Plan §5.1). Taslak ihtiyacı cihaz
tarafında (offline kuyruk + form durumu) zaten karşılanır; sunucu tarafı
taslak, kanıt niteliğini sulandırır ve kilit modelini karmaşıklaştırırdı.
Skor sunucuda hesaplanır — istemciden gelen skora güvenilmez.

## 4. Kesinti önerisi "çekme" modeliyle: hakediş taslağı öneriyi LİSTELER,
PY ekler; otomatik yazma yok

**Karar:** Bekleyen para cezaları taslak hakediş GET yanıtında
`ohs_penalty_suggestions` olarak döner; PY öneriyi mevcut taslak düzenleme
akışıyla (`deductions[]`, artık `source_entity/source_id` taşır) ekler.
Hakediş Finalize transaction'ı, `source_id`'li kesintilerin tutanaklarını
`AppliedToPayment + applied_payment_id` yapar.

**Gerekçe:** Plan Faz 8: "otomatik ÖNERİLEN kesinti, PY ONAYIYLA uygulanır".
Taslağa otomatik satır yazmak iki sorun doğururdu: (a) taslak kesintileri
her kayıtta istemci gövdesinden yeniden yazılır — otomatik satır sessizce
silinebilirdi; (b) ceza, hakediş açıldıktan SONRA kesilirse yine öneri
mekanizması gerekirdi. Çekme modeli tek ve tutarlı yoldur; `already_added`
bayrağı ve Finalize'daki statü geçişi çift kesintiyi engeller. Statü geri
alınamazlığı DB trigger'ındadır.

## 5. Tutanak numarası `ISG-NNN`, advisory lock ile

**Karar:** PR/PO deseniyle aynı: `pg_advisory_xact_lock(hashtext('ohs_penalty_no:'||pid))`
altında `MAX+1`.

**Gerekçe:** Tutanak resmi belge niteliğindedir; boşluksuz/yarışsız sıra
beklentisi PR numarasındakiyle aynıdır, yeni desen icat edilmedi.

## 6. Termin bildirimi bulgu başına BİR kez (`overdue_notified_at`)

**Karar:** Worker saatlik taramada süresi geçen açık bulguları raporlayana
bildirir ve işaretler; görev deadline'ındaki (Faz 4) desenle aynı.

**Gerekçe:** Tekrarlayan bildirim gürültüsü bildirimlerin görmezden
gelinmesine yol açar. Yaşlandırma zaten listede (`age_days`) ve Faz 9
dashboard'unda görünür olacak.
