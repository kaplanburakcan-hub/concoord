-- Migration 000049 — Makine/Ekipman/Araç Envanteri Faz B: proje-arası
-- transfer talebi + tek aşamalı onay. payment_plan_change_requests ile
-- aynı kalıp (bkz. migration 000043).

CREATE TABLE equipment_transfer_requests (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_equipment_id  uuid NOT NULL REFERENCES company_equipment(id) ON DELETE CASCADE,
    from_project_id       uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    to_project_id         uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    requested_by          uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status                text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    decided_by            uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    decided_at            timestamptz NULL,
    decision_note         text NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    row_version           integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_etr_from_status ON equipment_transfer_requests (from_project_id, status);
CREATE INDEX idx_etr_equipment ON equipment_transfer_requests (company_equipment_id);
-- Aynı ekipman için aynı anda birden fazla bekleyen talep olamaz.
CREATE UNIQUE INDEX idx_etr_equipment_pending ON equipment_transfer_requests (company_equipment_id)
    WHERE status = 'pending';
