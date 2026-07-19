-- Faz 11 / 000014 geri alma.
ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS progress_payments_status_check;
UPDATE progress_payments SET status = 'SiteApproved' WHERE status = 'InApproval';
ALTER TABLE progress_payments
    ADD CONSTRAINT progress_payments_status_check
    CHECK (status IN ('Draft','Submitted','SiteApproved','Finalized','Rejected'));

ALTER TABLE progress_payments DROP COLUMN IF EXISTS current_step_no;

DROP TABLE IF EXISTS payment_approvals;
DROP TABLE IF EXISTS payment_approval_steps;

ALTER TABLE deduction_catalog
    DROP COLUMN IF EXISTS default_vat_pct,
    DROP COLUMN IF EXISTS budget_code;

ALTER TABLE payment_deductions
    DROP COLUMN IF EXISTS net_amount,
    DROP COLUMN IF EXISTS vat_pct;

ALTER TABLE progress_payments
    DROP COLUMN IF EXISTS actual_cost,
    DROP COLUMN IF EXISTS payable_gross,
    DROP COLUMN IF EXISTS vat_collected,
    DROP COLUMN IF EXISTS vat_withheld,
    DROP COLUMN IF EXISTS vat_amount;
