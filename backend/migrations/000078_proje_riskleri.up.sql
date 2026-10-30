-- Faz 3.2 (E3) — proje risk kaydı. Kayıt silinmez; durum değişir.
-- Skor = olasılık × etki (1-25). Kapanmış riskler de listede kalır.
CREATE TABLE project_risks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    risk_no      integer NOT NULL,
    title        text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    description  text NULL,
    probability  integer NOT NULL CHECK (probability BETWEEN 1 AND 5),
    impact       integer NOT NULL CHECK (impact BETWEEN 1 AND 5),
    owner_id     uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'Açık'
                 CHECK (status IN ('Açık','İzleniyor','Kapandı')),
    created_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1,
    CONSTRAINT uq_project_risk_no UNIQUE (project_id, risk_no)
);
CREATE INDEX idx_project_risks_project ON project_risks (project_id, status) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_project_risks_updated_at BEFORE UPDATE ON project_risks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
