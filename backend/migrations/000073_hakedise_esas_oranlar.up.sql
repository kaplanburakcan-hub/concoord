-- Hakedişe esas vergi ve kesinti oranları: sözleşme girişinde belirlenir ve
-- orada kilitlenir; hakedişler bu oranları varsayılan alır. Hakedişte
-- değiştirmek progress_payments.edit_rates izni gerektirir (uygulama katmanı).
--
-- is_turu: yapım / hizmet / kiralama gibi iş türü (oranların neden farklı
-- olduğunu belgeler; oranlar tür seçimiyle otomatik değişmez).

-- Ana Sözleşme (işveren ↔ yüklenici) — İdari Hakediş'in oran kaynağı
ALTER TABLE project_main_contracts
    ADD COLUMN IF NOT EXISTS is_turu           text         NOT NULL DEFAULT 'yapim'
        CHECK (is_turu IN ('yapim','hizmet','kiralama','diger')),
    ADD COLUMN IF NOT EXISTS kdv_pct           numeric(5,2) NOT NULL DEFAULT 20
        CHECK (kdv_pct >= 0 AND kdv_pct <= 100),
    ADD COLUMN IF NOT EXISTS kdv_tevkifat_pct  numeric(5,2) NOT NULL DEFAULT 0
        CHECK (kdv_tevkifat_pct >= 0 AND kdv_tevkifat_pct <= 100),
    ADD COLUMN IF NOT EXISTS stopaj_pct        numeric(6,3) NOT NULL DEFAULT 0
        CHECK (stopaj_pct >= 0 AND stopaj_pct <= 100),
    ADD COLUMN IF NOT EXISTS damga_pct         numeric(6,3) NOT NULL DEFAULT 0
        CHECK (damga_pct >= 0 AND damga_pct <= 100),
    ADD COLUMN IF NOT EXISTS teminat_pct       numeric(5,2) NOT NULL DEFAULT 0
        CHECK (teminat_pct >= 0 AND teminat_pct <= 100);

-- Taşeron sözleşmeleri (retention_pct ve withholding_pct zaten var)
ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS is_turu               text         NOT NULL DEFAULT 'yapim'
        CHECK (is_turu IN ('yapim','hizmet','kiralama','diger')),
    ADD COLUMN IF NOT EXISTS vat_pct               numeric(5,2) NOT NULL DEFAULT 20
        CHECK (vat_pct >= 0 AND vat_pct <= 100),
    ADD COLUMN IF NOT EXISTS vat_withholding_ratio numeric(6,4) NOT NULL DEFAULT 0
        CHECK (vat_withholding_ratio >= 0 AND vat_withholding_ratio <= 1);

-- Taşeron hakedişinde teminat / stopaj oranı sözleşmeden farklıysa (yetkili
-- kullanıcı değiştirdiyse) hakedişe özel değer; NULL = sözleşmeden gelir.
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS retention_pct_override   numeric(5,2) NULL
        CHECK (retention_pct_override IS NULL OR (retention_pct_override >= 0 AND retention_pct_override <= 100)),
    ADD COLUMN IF NOT EXISTS withholding_pct_override numeric(5,2) NULL
        CHECK (withholding_pct_override IS NULL OR (withholding_pct_override >= 0 AND withholding_pct_override <= 100));
