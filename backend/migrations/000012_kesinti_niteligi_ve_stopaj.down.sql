-- Faz 11 / 000012 geri alma.
ALTER TABLE progress_payments DROP COLUMN IF EXISTS withholding_applied;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_withholding;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS withholding_pct,
    DROP COLUMN IF EXISTS is_multi_year;

DROP INDEX IF EXISTS idx_pd_reduces_cost;
ALTER TABLE payment_deductions DROP CONSTRAINT IF EXISTS chk_pd_nature;
ALTER TABLE payment_deductions
    DROP COLUMN IF EXISTS reduces_cost,
    DROP COLUMN IF EXISTS nature;

-- Stopaj tipini kaldırmadan önce kalan kayıtları genel vergiye çevir.
UPDATE payment_deductions SET type = 'Tax' WHERE type = 'Withholding';
ALTER TABLE payment_deductions DROP CONSTRAINT IF EXISTS payment_deductions_type_check;
ALTER TABLE payment_deductions
    ADD CONSTRAINT payment_deductions_type_check
    CHECK (type IN ('AdvanceOffset','Retention','Tax','OHSPenalty','Other'));
