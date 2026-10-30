-- Faz 3.2 (E3) — karar günlüğü. Kayıt değiştirilmez; yeni karar eskisini
-- 'supersedes_id' ile geçersiz kılar. Aktif kararlar = başka bir kayıt
-- tarafından geçersiz kılınmamış olanlar.
CREATE TABLE project_decisions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    meeting_id    uuid NULL REFERENCES project_meetings (id) ON DELETE RESTRICT,
    decision_text text NOT NULL CHECK (char_length(btrim(decision_text)) BETWEEN 1 AND 2000),
    decided_by    uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    decided_on    date NOT NULL,
    supersedes_id uuid NULL REFERENCES project_decisions (id) ON DELETE RESTRICT,
    created_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_decision_no_self_supersede CHECK (supersedes_id IS NULL OR supersedes_id <> id)
);
CREATE INDEX idx_project_decisions_project ON project_decisions (project_id, decided_on);
CREATE INDEX idx_project_decisions_supersedes ON project_decisions (supersedes_id);
-- Bir karar yalnız bir kez geçersiz kılınabilir (dallanan zincir olmaz).
CREATE UNIQUE INDEX uq_project_decision_supersedes ON project_decisions (supersedes_id) WHERE supersedes_id IS NOT NULL;
-- Geçersiz kılma yalnız aynı projedeki bir kararı hedefleyebilir (uygulama katmanında da denetlenir).
CREATE OR REPLACE FUNCTION check_decision_supersedes_project() RETURNS trigger AS $$
BEGIN
    IF NEW.supersedes_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM project_decisions d WHERE d.id = NEW.supersedes_id AND d.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'geçersiz kılınan karar başka projeye ait';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_decision_supersedes_project BEFORE INSERT ON project_decisions
    FOR EACH ROW EXECUTE FUNCTION check_decision_supersedes_project();
-- Karar değiştirilemez: UPDATE ve DELETE engellenir.
CREATE OR REPLACE FUNCTION forbid_decision_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'karar kayıtları değiştirilemez';
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_project_decisions_immutable BEFORE UPDATE OR DELETE ON project_decisions
    FOR EACH ROW EXECUTE FUNCTION forbid_decision_mutation();
