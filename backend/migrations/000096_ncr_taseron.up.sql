-- NCR ↔ taşeron (E14, taşeron görünümü). Taşeron temsilcisi yalnız kendi firmasına
-- bağlı uygunsuzlukları görür. Taşeron aynı projeye ait olmalıdır (tetikleyici).
ALTER TABLE ncrs
    ADD COLUMN subcontractor_id uuid NULL REFERENCES subcontractors (id) ON DELETE RESTRICT;
CREATE INDEX idx_ncrs_subcontractor ON ncrs (subcontractor_id) WHERE subcontractor_id IS NOT NULL;
CREATE OR REPLACE FUNCTION check_ncr_subcontractor_project() RETURNS trigger AS $$
BEGIN
    IF NEW.subcontractor_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM subcontractors s WHERE s.id = NEW.subcontractor_id AND s.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'taşeron bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_ncr_subcontractor_project BEFORE INSERT OR UPDATE OF subcontractor_id, project_id ON ncrs
    FOR EACH ROW EXECUTE FUNCTION check_ncr_subcontractor_project();
