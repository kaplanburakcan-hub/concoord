-- NCR kapanış kanıtı (E4). Kapatılan kayıt, aynı projeye ait bir belgeye bağlanır.
-- Mevcut kapalı kayıtlar kanıtsız kalır (geriye dönük zorunluluk yok); yeni kapatmalarda
-- uygulama katmanı belge ister.
ALTER TABLE ncrs
    ADD COLUMN closed_document_id uuid NULL REFERENCES documents (id) ON DELETE RESTRICT;
CREATE OR REPLACE FUNCTION check_ncr_closed_doc_project() RETURNS trigger AS $$
BEGIN
    IF NEW.closed_document_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM documents d WHERE d.id = NEW.closed_document_id AND d.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'kapanış belgesi bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_ncr_closed_doc_project BEFORE INSERT OR UPDATE OF closed_document_id, project_id ON ncrs
    FOR EACH ROW EXECUTE FUNCTION check_ncr_closed_doc_project();
