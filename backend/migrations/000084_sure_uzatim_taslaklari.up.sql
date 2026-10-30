-- Faz 3.4 (E2) — süre uzatımı (EOT) talebi, ilk adım: yalnız taslak kayıt.
-- Onay, sözleşme bitiş tarihi güncellemesi ve iş programı etkisi bu aşamada
-- YOKTUR. Kayıt silinmez.
CREATE TABLE time_extension_drafts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    draft_no     integer NOT NULL,
    title        text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    reason_code  text NOT NULL CHECK (reason_code IN ('Sahip','Hava','Tasarim','MucbirSebep','Diger')),
    requested_days integer NOT NULL CHECK (requested_days BETWEEN 1 AND 3650),
    basis        text NULL,
    status       text NOT NULL DEFAULT 'Taslak' CHECK (status IN ('Taslak')),
    created_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    row_version  integer NOT NULL DEFAULT 1,
    CONSTRAINT uq_time_extension_no UNIQUE (project_id, draft_no)
);
CREATE INDEX idx_time_extension_project ON time_extension_drafts (project_id);
