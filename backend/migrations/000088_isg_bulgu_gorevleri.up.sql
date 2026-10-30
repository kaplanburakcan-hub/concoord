-- Faz 8 / B11 — İSG bulgusundan düzeltici görev. Bir bulgu en fazla bir görevle
-- eşleşir (birincil anahtar + tekil görev). Bulgu kaydı değişmez.
CREATE TABLE ohs_finding_tasks (
    finding_id uuid PRIMARY KEY REFERENCES ohs_findings (id) ON DELETE RESTRICT,
    task_id    uuid NOT NULL REFERENCES tasks (id) ON DELETE RESTRICT,
    created_by uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_ohs_finding_task UNIQUE (task_id)
);
