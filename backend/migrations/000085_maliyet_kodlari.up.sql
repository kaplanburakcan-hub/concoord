-- Faz 3.6 (E6) — maliyet kodu tanım listesi (ilk adım). Bütçe, PO ve hakediş ile
-- bağlantı bu aşamada YOKTUR. Kayıt silinmez; yalnız eklenir.
CREATE TABLE cost_codes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    code        text NOT NULL CHECK (char_length(btrim(code)) BETWEEN 1 AND 40),
    name        text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    parent_id   uuid NULL REFERENCES cost_codes (id) ON DELETE RESTRICT,
    created_by  uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_cost_code UNIQUE (project_id, code),
    CONSTRAINT chk_cost_code_no_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX idx_cost_codes_project ON cost_codes (project_id, code);
-- Üst kod aynı projeye ait olmalı.
CREATE OR REPLACE FUNCTION check_cost_code_parent_project() RETURNS trigger AS $$
BEGIN
    IF NEW.parent_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM cost_codes c WHERE c.id = NEW.parent_id AND c.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'üst maliyet kodu başka projeye ait';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_cost_code_parent_project BEFORE INSERT ON cost_codes
    FOR EACH ROW EXECUTE FUNCTION check_cost_code_parent_project();
