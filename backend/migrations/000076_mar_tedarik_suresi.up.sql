-- Faz 3.1 (E7) — malzeme onayında tedarik süresi ve ihtiyaç tarihi.
-- Sipariş son tarihi = needed_by_date - lead_time_days (uygulama katmanında hesaplanır;
-- takvim veya iş programı değiştirilmez).
ALTER TABLE material_approvals
    ADD COLUMN lead_time_days  integer NULL CHECK (lead_time_days IS NULL OR (lead_time_days >= 0 AND lead_time_days <= 365)),
    ADD COLUMN needed_by_date  date    NULL;
