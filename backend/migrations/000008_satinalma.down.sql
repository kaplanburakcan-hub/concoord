-- Faz 7 geri alma — satınalma tabloları ve kilit fonksiyonları.
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS purchase_orders;

DROP TRIGGER IF EXISTS trg_pri_lock ON purchase_request_items;
DROP TRIGGER IF EXISTS trg_pr_lock ON purchase_requests;
DROP FUNCTION IF EXISTS lock_nondraft_pr_items();
DROP FUNCTION IF EXISTS lock_decided_purchase_request();

DROP TABLE IF EXISTS purchase_request_items;
DROP TABLE IF EXISTS purchase_requests;
