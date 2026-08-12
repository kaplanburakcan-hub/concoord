-- Migration 000043 — Nakit Akış Faz C: ödeme planı değişikliği onay akışı.
--
-- Bir ödeme planı satırı (hakediş/ekstre/PO ödemesi) sözleşme/tedarikçi
-- varsayılan ödeme şeklinden FARKLI bir şekille girilirse, satır DB'ye
-- yazılır ama cash_events'e yazılmaz — bunun yerine burada bir onay
-- talebi açılır. Onaylanırsa istenen şekille, reddedilirse varsayılan
-- şekille cash_events satırı üretilir.

CREATE TABLE payment_plan_change_requests (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    source_entity    text NOT NULL, -- 'progress_payment_disbursement' | 'supplier_payment' | 'po_payment'
    source_id        uuid NOT NULL,
    amount_snapshot      numeric(18,2) NOT NULL,
    description_snapshot text NOT NULL,
    default_method   text NOT NULL CHECK (default_method IN ('nakit','havale','cek')),
    requested_method text NOT NULL CHECK (requested_method IN ('nakit','havale','cek')),
    requested_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    decided_by       uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    decided_at       timestamptz NULL,
    decision_note    text NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    row_version      integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ppcr_project_status ON payment_plan_change_requests (project_id, status);
CREATE UNIQUE INDEX idx_ppcr_source_pending ON payment_plan_change_requests (source_entity, source_id)
    WHERE status = 'pending';
