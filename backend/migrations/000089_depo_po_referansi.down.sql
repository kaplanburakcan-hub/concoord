-- Depo PO referansı — geri alma
DROP TRIGGER IF EXISTS trg_warehouse_po_project ON site_warehouse_movements;
DROP FUNCTION IF EXISTS check_warehouse_po_project();
DROP INDEX IF EXISTS idx_warehouse_movements_po;
ALTER TABLE site_warehouse_movements DROP COLUMN IF EXISTS purchase_order_id;
