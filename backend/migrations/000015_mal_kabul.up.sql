-- ============================================================================
-- Faz 11 / 000015 — Mal kabul: kalem bazında teslim alma, eksik/hasar bildirimi
-- ============================================================================
-- Faz 7'de `deliveries` yalnızca irsaliye başlığını tutuyordu (irsaliye no,
-- tarih, teslim alan, not). Sahada asıl ihtiyaç bundan fazlası:
--
--   · Mal NEREYE girdi? Depoya mı, doğrudan sahaya mı, taşerona mı?
--   · Sipariş edilen miktarın NE KADARI geldi? (kısmi teslimat olağandır)
--   · Gelen mal SAĞLAM MI? Eksik, hasarlı veya şartnameye aykırı olabilir;
--     bu durumda kabul edilen ve reddedilen miktar ayrı ayrı kaydedilmelidir.
--
-- Uygunsuzluk kaydı ileride tedarikçiye iade, eksik tamamlama veya hakedişten
-- kesinti dayanağı olur; bu yüzden fotoğraf ve açıklama ile birlikte tutulur.
--
-- Sipariş durumu (Ordered → PartiallyDelivered → Delivered) teslim alınan
-- miktarlara göre uygulama katmanında güncellenir.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1) Teslimat başlığı: yer, durum, uygunsuzluk
-- ---------------------------------------------------------------------------
ALTER TABLE deliveries
    ADD COLUMN IF NOT EXISTS receipt_type       text NOT NULL DEFAULT 'Warehouse',
    ADD COLUMN IF NOT EXISTS location_note      text NULL,
    ADD COLUMN IF NOT EXISTS condition          text NOT NULL DEFAULT 'Complete',
    ADD COLUMN IF NOT EXISTS discrepancy_note   text NULL,
    ADD COLUMN IF NOT EXISTS photo_document_id  uuid NULL REFERENCES documents (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS inspected_by       uuid NULL REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_receipt_type;
ALTER TABLE deliveries
    ADD CONSTRAINT chk_deliveries_receipt_type
    CHECK (receipt_type IN ('Warehouse','Site','DirectToSubcontractor'));

ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_condition;
ALTER TABLE deliveries
    ADD CONSTRAINT chk_deliveries_condition
    CHECK (condition IN ('Complete','Short','Damaged','Defective','Rejected'));

-- Uygunsuz teslimatta açıklama zorunludur: "eksik geldi" denip gerekçesiz
-- bırakılan kayıt sonradan tedarikçiyle yapılacak görüşmede işe yaramaz.
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_discrepancy;
ALTER TABLE deliveries
    ADD CONSTRAINT chk_deliveries_discrepancy CHECK (
        condition = 'Complete'
        OR (discrepancy_note IS NOT NULL AND length(btrim(discrepancy_note)) > 0)
    );

COMMENT ON COLUMN deliveries.receipt_type IS
    'Warehouse=depo girişi, Site=doğrudan sahaya, DirectToSubcontractor=taşerona teslim';
COMMENT ON COLUMN deliveries.condition IS
    'Complete=tam ve sağlam, Short=eksik, Damaged=hasarlı, Defective=şartnameye aykırı, Rejected=tümü reddedildi';

-- ---------------------------------------------------------------------------
-- 2) Teslimat kalemleri
--
-- Kalem, mümkünse satınalma talebi kalemine bağlanır (izlenebilirlik); serbest
-- kalem de girilebilir çünkü sahada sipariş dışı teslimat olabilir.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS delivery_items (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id   uuid NOT NULL REFERENCES deliveries (id) ON DELETE CASCADE,
    pr_item_id    uuid NULL REFERENCES purchase_request_items (id) ON DELETE RESTRICT,
    material_name text NOT NULL,
    unit          text NOT NULL DEFAULT 'ad',
    ordered_qty   numeric(14,3) NULL CHECK (ordered_qty  IS NULL OR ordered_qty >= 0),
    received_qty  numeric(14,3) NOT NULL CHECK (received_qty >= 0),
    accepted_qty  numeric(14,3) NOT NULL CHECK (accepted_qty >= 0),
    rejected_qty  numeric(14,3) NOT NULL DEFAULT 0 CHECK (rejected_qty >= 0),
    note          text NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_di_qty CHECK (accepted_qty + rejected_qty <= received_qty + 0.001)
);
CREATE INDEX IF NOT EXISTS idx_delivery_items_delivery ON delivery_items (delivery_id);
CREATE INDEX IF NOT EXISTS idx_delivery_items_pr_item  ON delivery_items (pr_item_id)
    WHERE pr_item_id IS NOT NULL;

DROP TRIGGER IF EXISTS trg_delivery_items_updated_at ON delivery_items;
CREATE TRIGGER trg_delivery_items_updated_at BEFORE UPDATE ON delivery_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON COLUMN delivery_items.received_qty IS 'Fiilen gelen miktar';
COMMENT ON COLUMN delivery_items.accepted_qty IS 'Kabul edilip stoğa/sahaya alınan miktar';
COMMENT ON COLUMN delivery_items.rejected_qty IS 'Hasarlı/hatalı olduğu için reddedilen, iade edilecek miktar';
