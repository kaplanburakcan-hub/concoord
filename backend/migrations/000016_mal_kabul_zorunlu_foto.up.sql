-- ============================================================================
-- Faz 11 / 000016 — Mal kabulde zorunlu iki fotoğraf
-- ============================================================================
-- Mal kabul kaydı iki ayrı kanıt gerektirir ve ikisi de ZORUNLUDUR:
--
--   1. İrsaliye fotoğrafı  (deliveries.document_id)
--      Tedarikçinin beyanı: ne gönderdiğini iddia ediyor.
--   2. Malzeme fotoğrafı   (deliveries.material_photo_document_id)
--      Sahanın tespiti: fiilen ne geldiği, hangi durumda olduğu.
--
-- İkisi birlikte, sonradan çıkacak eksik/hasar tartışmasında karşılaştırılabilir
-- bir kayıt oluşturur. Tek başına irsaliye yetersizdir (kâğıt üzerinde tam
-- görünen teslimat fiilen hasarlı gelebilir); tek başına malzeme fotoğrafı da
-- yetersizdir (neyin sipariş edildiği belgelenmemiş olur).
--
-- ZORUNLULUK UYGULAMA KATMANINDA denetlenir, veritabanı NOT NULL ile değil:
-- Faz 7'den kalan mevcut teslimat kayıtlarında bu alanlar boştur ve geriye dönük
-- fotoğraf üretmek mümkün değildir. NOT NULL koymak eski kayıtları geçersiz
-- kılardı; kayıt defteri bütünlüğü ilkesi gereği eski veri bozulmaz.
-- ============================================================================

ALTER TABLE deliveries
    ADD COLUMN IF NOT EXISTS material_photo_document_id uuid NULL
        REFERENCES documents (id) ON DELETE RESTRICT;

COMMENT ON COLUMN deliveries.document_id IS
    'İrsaliye fotoğrafı/belgesi — yeni kayıtlarda zorunlu (uygulama katmanı)';
COMMENT ON COLUMN deliveries.material_photo_document_id IS
    'Teslim alınan malzemenin fotoğrafı — yeni kayıtlarda zorunlu (uygulama katmanı)';

-- 000015'te eklenen photo_document_id, uygunsuzluk (eksik/hasar) kanıtı olarak
-- OPSİYONEL kalır: malzeme fotoğrafı zaten zorunlu olduğundan bu alan yalnızca
-- ek detay (yakın çekim, hasarlı ambalaj vb.) için kullanılır.
COMMENT ON COLUMN deliveries.photo_document_id IS
    'Uygunsuzluk ek fotoğrafı (opsiyonel) — hasar/eksik detayı';
