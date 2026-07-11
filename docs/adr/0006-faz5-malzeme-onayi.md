# ADR 0006 — Faz 5: Malzeme Onay Süreci (Submittals / MAR)

**Tarih:** 07.07.2026 · **Durum:** Kabul edildi · **Plan referansı:** §6.5, §8 (Faz 5)

## Bağlam
Faz 5, malzeme onay (submittal) sürecini platforma taşır: MAR formu, doküman
ekleri, statü akışı, renk kodlu durum panosu, karar notu zorunluluğu,
müşavir/işveren (Client) kısıtlı inceleme ekranı ve MAR kayıt defteri dışa
aktarımı. Faz 2 doküman motoru ve Faz 4 bildirim motoru bilinçli olarak önce
inşa edildiği için bu faz ikisini de tüketir, kendi altyapısını yazmaz (Plan §9).

## Kararlar

### 1. Yaşam döngüsü: `Submitted → UnderReview → {Approved | ConditionallyApproved | Rejected}`
Plan §6.5'teki statü kümesi aynen kullanıldı; ayrıca Draft statüsü EKLENMEDİ —
MAR oluşturulduğu anda sunulmuştur (Submitted). Metraj gibi iteratif bir hazırlık
içermediğinden taslak katmanı gereksiz karmaşıklık sayıldı; Submitted durumunda
künye hâlâ düzenlenebilir. Her geçiş `workflow_transitions`a yazılır (Plan §5.1).

### 2. "Kendisine sunulan" = UnderReview ve sonrası (Client kısıtlı görünüm)
`Submitted` kayıtlar iç incelemededir; Client bunları GÖREMEZ. Saha/PY tarafı
`review` aksiyonuyla kaydı `UnderReview` yapar — bu, MAR'ı müşavire/işverene
"sunma" anıdır. Filtre backend'de zorunludur: liste, tekil kayıt ve CSV dışa
aktarımı aynı kapsam koşullarını paylaşır; Client Submitted bir MAR'ın id'sini
bilse dahi 404 alır (bilgi sızıntısı yok). Aynı ilke SubcontractorRep taşeron
kapsamı için de geçerlidir (Plan §4 satır seviyesi güvenlik).

### 3. Karar notu zorunluluğu iki katmanlı
Uygulama katmanı (validateDecision) + veritabanı CHECK kısıtı
(`chk_mar_decision_note`): karara bağlanmış bir satırda `decision_note` boş
olamaz. Karar veren ve karar anı (`decided_by`, `decided_at`) kayda işlenir;
audit log ve workflow_transitions ile birlikte "kim, ne zaman, hangi notla"
her zaman yanıtlanır.

### 4. Doküman ekleri polimorfik motor üzerinden
Yeni tablo yok: `documents.entity_type='material_approval'`, `entity_id=MAR id`,
kategori `Submittal`. Versiyonlama, SHA-256 ve erişim denetimi Faz 2'den hazır
gelir. Liste/detay yanıtları `attachment_count` alt sorgusuyla ek sayısını taşır.

### 5. MAR numaralama: proje içi sıralı, advisory lock ile
`MAR-001, MAR-002...` proje kapsamında üretilir; eşzamanlı oluşturmalarda
`pg_advisory_xact_lock` yarışı serileştirir, `(project_id, mar_no)` tekil
indeksi son sigortadır. Soft-delete edilen kayıt numarasını geri bırakmaz —
kayıt defteri numara sürekliliği korunur.

### 6. Bildirim hedefleme rol varsayılanı üzerinden
- Yeni MAR (Submitted) → projede `material_approvals.review` yetkisini rolü
  gereği taşıyan üyeler (Client hariç — henüz sunulmadı).
- Karara sunma (UnderReview) → `material_approvals.decide` taşıyanlar (Client dahil).
- Karar → talep sahibi + ilgili taşeronun temsilcileri.

Hedefleme rol VARSAYILANLARINDAN okunur (rbac.RoleHasDefault); kullanıcı bazlı
override'lar bildirim hedeflemesine yansıtılmaz. Gerekçe: override'lar erişim
kontrolünde zaten kesin uygulanır (API katmanı), bildirim ise yan etkidir —
üye başına tam izin çözümü sorgu maliyeti getirirdi. Kabul edilen sınırlılık:
DENY override'lı bir kullanıcı bildirimi alabilir ama kaydı açamaz.

### 7. Kayıt defteri dışa aktarımı: CSV (`;` ayırıcı + UTF-8 BOM)
Türkçe bölge ayarlı Excel'de doğrudan açılır. Dışa aktarım, isteği yapan
kullanıcının kapsam filtreleriyle üretilir — dışa aktarım bir yetki kaçış
yolu değildir.

## Kabul kriterleri karşılığı (Plan Faz 5)
- *"Client rolü yalnızca kendisine sunulan MAR'ları görüp karar verebiliyor"*
  → Karar 2 (backend zorunlu filtre) + `material_approvals.decide` Client rol
  varsayılanında mevcut; karar ucu yalnızca UnderReview kayıtlarda çalışır.
- *"Karar bildirimi ilgililere düşüyor"* → Karar 6; merkezi notify servisi
  in-app + (tercihe göre) e-posta/SMS kanallarını kullanır.

## Sonuçlar
- Yeni migration: `000006_malzeme_onayi` (tek tablo + kısıtlar).
- Yeni paket: `backend/internal/materials` (handler, validate + birim testleri, register).
- Faz 8 İSG ve Faz 6 haftalık rapor, MAR özetlerini bu tablodan derleyecek.
