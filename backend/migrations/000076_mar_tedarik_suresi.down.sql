-- Faz 3.1 (E7) — geri alma
ALTER TABLE material_approvals DROP COLUMN IF EXISTS needed_by_date, DROP COLUMN IF EXISTS lead_time_days;
