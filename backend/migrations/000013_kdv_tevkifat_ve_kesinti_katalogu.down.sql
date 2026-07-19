-- Faz 11 / 000013 geri alma.
DROP INDEX IF EXISTS idx_pd_group;
ALTER TABLE payment_deductions
    DROP COLUMN IF EXISTS catalog_code,
    DROP COLUMN IF EXISTS group_code;

DROP TABLE IF EXISTS deduction_catalog;

ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS excl_pp_period_overlap;
ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS chk_pp_period_order;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_dates;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS revised_end_date,
    DROP COLUMN IF EXISTS end_date,
    DROP COLUMN IF EXISTS start_date;

ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS chk_pp_vat_withholding;
ALTER TABLE progress_payments
    DROP COLUMN IF EXISTS vat_exemption_code,
    DROP COLUMN IF EXISTS vat_withholding_ratio;
