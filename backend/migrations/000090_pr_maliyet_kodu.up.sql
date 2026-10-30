-- Satın alma talebine (PR) isteğe bağlı maliyet kodu (E8 / E6). Mevcut talepler boş kalır.
-- Kod aynı projeye ait olmalıdır (tetikleyici). PR'dan oluşturulan sipariş bu kodu
-- devralabilir (uygulama katmanında).
ALTER TABLE purchase_requests
    ADD COLUMN cost_code_id uuid NULL REFERENCES cost_codes (id) ON DELETE RESTRICT;
CREATE INDEX idx_purchase_requests_cost_code ON purchase_requests (cost_code_id) WHERE cost_code_id IS NOT NULL;
CREATE OR REPLACE FUNCTION check_pr_cost_code_project() RETURNS trigger AS $$
BEGIN
    IF NEW.cost_code_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM cost_codes c WHERE c.id = NEW.cost_code_id AND c.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'maliyet kodu bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_pr_cost_code_project BEFORE INSERT OR UPDATE OF cost_code_id, project_id ON purchase_requests
    FOR EACH ROW EXECUTE FUNCTION check_pr_cost_code_project();
