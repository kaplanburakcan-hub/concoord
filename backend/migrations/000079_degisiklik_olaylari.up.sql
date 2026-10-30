-- Faz 3.3 (E1) — değişiklik olayı (ilk adım: yalnız taslak kayıt).
-- Onay akışı, sözleşme bedeli ve bütçe güncellemesi bu aşamada YOKTUR.
CREATE TABLE change_events (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    event_no         integer NOT NULL,
    title            text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    source           text NOT NULL
                     CHECK (source IN ('Sahip','Tasarim','SahaKosulu','RFI','HakTalebi','Diger')),
    description      text NULL,
    estimated_amount numeric(14,2) NULL CHECK (estimated_amount IS NULL OR estimated_amount >= 0),
    estimated_days   integer NULL CHECK (estimated_days IS NULL OR (estimated_days >= 0 AND estimated_days <= 3650)),
    status           text NOT NULL DEFAULT 'Taslak' CHECK (status IN ('Taslak')),
    created_by       uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1,
    CONSTRAINT uq_change_event_no UNIQUE (project_id, event_no)
);
CREATE INDEX idx_change_events_project ON change_events (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_change_events_updated_at BEFORE UPDATE ON change_events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
