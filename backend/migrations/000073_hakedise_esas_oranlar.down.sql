ALTER TABLE progress_payments
    DROP COLUMN IF EXISTS retention_pct_override,
    DROP COLUMN IF EXISTS withholding_pct_override;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS is_turu,
    DROP COLUMN IF EXISTS vat_pct,
    DROP COLUMN IF EXISTS vat_withholding_ratio;
ALTER TABLE project_main_contracts
    DROP COLUMN IF EXISTS is_turu,
    DROP COLUMN IF EXISTS kdv_pct,
    DROP COLUMN IF EXISTS kdv_tevkifat_pct,
    DROP COLUMN IF EXISTS stopaj_pct,
    DROP COLUMN IF EXISTS damga_pct,
    DROP COLUMN IF EXISTS teminat_pct;
