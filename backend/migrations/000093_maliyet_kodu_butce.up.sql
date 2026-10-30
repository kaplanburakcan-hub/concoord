-- Maliyet kodu başına tek planlanan bütçe tutarı (E6, ilk adım). Yalnız giriş:
-- EVM, sapma ve PO/hakediş hesabı bu aşamada bu tabloyu kullanmaz.
CREATE TABLE cost_code_budgets (
    cost_code_id uuid PRIMARY KEY REFERENCES cost_codes (id) ON DELETE RESTRICT,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    amount       numeric(14,2) NOT NULL CHECK (amount >= 0),
    updated_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_cost_code_budgets_project ON cost_code_budgets (project_id);
CREATE OR REPLACE FUNCTION check_budget_project() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM cost_codes c WHERE c.id = NEW.cost_code_id AND c.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'bütçe kodu bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_cost_code_budget_project BEFORE INSERT OR UPDATE OF cost_code_id, project_id ON cost_code_budgets
    FOR EACH ROW EXECUTE FUNCTION check_budget_project();
