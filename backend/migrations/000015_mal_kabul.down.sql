-- Faz 11 / 000015 geri alma.
DROP TABLE IF EXISTS delivery_items;

ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_discrepancy;
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_condition;
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS chk_deliveries_receipt_type;

ALTER TABLE deliveries
    DROP COLUMN IF EXISTS inspected_by,
    DROP COLUMN IF EXISTS photo_document_id,
    DROP COLUMN IF EXISTS discrepancy_note,
    DROP COLUMN IF EXISTS condition,
    DROP COLUMN IF EXISTS location_note,
    DROP COLUMN IF EXISTS receipt_type;
