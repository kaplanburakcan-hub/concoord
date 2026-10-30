-- PR maliyet kodu — geri alma
DROP TRIGGER IF EXISTS trg_pr_cost_code_project ON purchase_requests;
DROP FUNCTION IF EXISTS check_pr_cost_code_project();
DROP INDEX IF EXISTS idx_purchase_requests_cost_code;
ALTER TABLE purchase_requests DROP COLUMN IF EXISTS cost_code_id;
