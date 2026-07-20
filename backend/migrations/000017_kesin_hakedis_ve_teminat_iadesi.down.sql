-- Faz 11 / 000017 geri alma.
DROP VIEW IF EXISTS v_retention_balance;
DROP TABLE IF EXISTS deduction_refunds;
DROP INDEX IF EXISTS uq_pp_final_per_sub;
ALTER TABLE progress_payments
    DROP COLUMN IF EXISTS total_additions,
    DROP COLUMN IF EXISTS provisional_acceptance_date,
    DROP COLUMN IF EXISTS provisional_acceptance_document_id,
    DROP COLUMN IF EXISTS is_final;
