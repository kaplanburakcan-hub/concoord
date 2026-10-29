-- Faz 3.1 — EVM planlanan değer (PV) kaynağı seçimi.
-- 'auto'     : mevcut öncelik (manuel aylık giriş → milestone ağırlığı → doğrusal).
-- 'baseline' : son dondurulmuş iş programı baseline'ından türetilir (baseline yoksa auto'ya düşer).
ALTER TABLE project_control_settings
    ADD COLUMN pv_source text NOT NULL DEFAULT 'auto'
        CHECK (pv_source IN ('auto', 'baseline'));
