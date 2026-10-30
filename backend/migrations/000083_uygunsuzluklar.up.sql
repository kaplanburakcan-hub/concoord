-- Faz 3.5 (E4) — uygunsuzluk (NCR) kaydı, ilk sürüm. Kayıt silinmez; kapanınca
-- durum 'Kapandı' olur ve kapanış bilgisi tutulur. Muayene ve hold noktaları
-- bu aşamada yoktur.
CREATE TABLE ncrs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    ncr_no       integer NOT NULL,
    title        text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    poz_no       text NULL,
    description  text NULL,
    owner_id     uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'Açık' CHECK (status IN ('Açık','Kapandı')),
    created_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    closed_by    uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    closed_at    timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1,
    CONSTRAINT uq_ncr_no UNIQUE (project_id, ncr_no),
    CONSTRAINT chk_ncr_closed CHECK ((status = 'Kapandı') = (closed_at IS NOT NULL))
);
CREATE INDEX idx_ncrs_project_status ON ncrs (project_id, status);
