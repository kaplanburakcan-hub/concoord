-- Faz 3.6 (E6 / B6) — sipariş satırına isteğe bağlı maliyet kodu. Mevcut siparişler
-- boş kalır. Maliyet kodu aynı projeye ait olmalıdır (tetikleyici). Bütçe hesabı bu
-- aşamada yoktur.
ALTER TABLE purchase_orders
    ADD COLUMN cost_code_id uuid NULL REFERENCES cost_codes (id) ON DELETE RESTRICT;
CREATE INDEX idx_purchase_orders_cost_code ON purchase_orders (cost_code_id) WHERE cost_code_id IS NOT NULL;
CREATE OR REPLACE FUNCTION check_po_cost_code_project() RETURNS trigger AS $$
BEGIN
    IF NEW.cost_code_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM cost_codes c WHERE c.id = NEW.cost_code_id AND c.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'maliyet kodu bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_po_cost_code_project BEFORE INSERT OR UPDATE OF cost_code_id, project_id ON purchase_orders
    FOR EACH ROW EXECUTE FUNCTION check_po_cost_code_project();
