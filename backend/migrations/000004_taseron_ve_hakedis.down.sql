-- Faz 3 / Migration 4 — geri alma (bağımlılık sırasına dikkat)

DROP TRIGGER IF EXISTS trg_pd_lock  ON payment_deductions;
DROP TRIGGER IF EXISTS trg_ppi_lock ON progress_payment_items;
DROP TRIGGER IF EXISTS trg_pp_lock  ON progress_payments;
DROP FUNCTION IF EXISTS lock_finalized_children();
DROP FUNCTION IF EXISTS lock_finalized_payment();

DROP TABLE IF EXISTS payment_deductions;
DROP TABLE IF EXISTS progress_payment_items;
DROP TABLE IF EXISTS progress_payments;
DROP TABLE IF EXISTS work_items;
DROP TABLE IF EXISTS contracts;

ALTER TABLE project_members DROP CONSTRAINT IF EXISTS fk_project_members_subcontractor;
DROP TABLE IF EXISTS subcontractors;
