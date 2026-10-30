-- Yazışma ↔ belge çok-çoka bağlantısı (B18). Mevcut belge eki (documents.entity_type/
-- entity_id) bire-birdir; bu tablo aynı belgenin birden çok yazışmaya bağlanmasını sağlar.
-- Belge ve yazışma değişmez. Her iki kayıt da aynı projeye ait olmalıdır (tetikleyici).
CREATE TABLE correspondence_document_links (
    correspondence_id uuid NOT NULL REFERENCES correspondences (id) ON DELETE RESTRICT,
    document_id       uuid NOT NULL REFERENCES documents (id) ON DELETE RESTRICT,
    created_by        uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (correspondence_id, document_id)
);
CREATE INDEX idx_corr_doc_links_doc ON correspondence_document_links (document_id);
CREATE OR REPLACE FUNCTION check_corr_doc_link_project() RETURNS trigger AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM correspondences c JOIN documents d ON d.project_id = c.project_id
        WHERE c.id = NEW.correspondence_id AND d.id = NEW.document_id
    ) THEN
        RAISE EXCEPTION 'yazışma ve belge aynı projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_corr_doc_link_project BEFORE INSERT ON correspondence_document_links
    FOR EACH ROW EXECUTE FUNCTION check_corr_doc_link_project();
