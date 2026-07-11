-- Faz 0 / Migration 1 — Temel altyapı
-- İlke (Plan §5.1): hiçbir veri kaybolmaz, hiçbir değişiklik izsiz kalmaz.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";  -- gen_random_uuid()

-- Merkezi denetim izi. Repository katmanı her yazma işleminde kayıt atar.
CREATE TABLE audit_logs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    uuid NULL,              -- FK Faz 1'de users tablosuna bağlanır
    entity      text NOT NULL,          -- örn. 'progress_payments'
    entity_id   uuid NULL,
    action      text NOT NULL CHECK (action IN ('INSERT','UPDATE','DELETE')),
    before      jsonb NULL,
    after       jsonb NULL,
    ip          text NULL,
    request_id  text NULL,
    at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_entity ON audit_logs (entity, entity_id);
CREATE INDEX idx_audit_logs_actor  ON audit_logs (actor_id, at DESC);
CREATE INDEX idx_audit_logs_at     ON audit_logs (at DESC);

-- Statülü varlıkların iş akışı geçiş logu ("kim, ne zaman, hangi notla").
CREATE TABLE workflow_transitions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity      text NOT NULL,
    entity_id   uuid NOT NULL,
    from_status text NULL,
    to_status   text NOT NULL,
    actor_id    uuid NULL,
    note        text NULL,
    at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_wf_transitions_entity ON workflow_transitions (entity, entity_id, at);

-- Ortak updated_at tetikleyicisi — sonraki migration'larda tüm iş tablolarına bağlanır.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
