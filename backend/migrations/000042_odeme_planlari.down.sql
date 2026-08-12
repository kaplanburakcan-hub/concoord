ALTER TABLE purchase_orders DROP COLUMN IF EXISTS tedarikci_id;
DROP TABLE IF EXISTS purchase_order_payments;
DROP TABLE IF EXISTS supplier_statement_payments;
DROP TABLE IF EXISTS progress_payment_disbursements;
