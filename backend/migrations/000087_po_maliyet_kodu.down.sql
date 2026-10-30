-- Faz 3.6 (E6 / B6) — geri alma
DROP TRIGGER IF EXISTS trg_po_cost_code_project ON purchase_orders;
DROP FUNCTION IF EXISTS check_po_cost_code_project();
DROP INDEX IF EXISTS idx_purchase_orders_cost_code;
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS cost_code_id;
