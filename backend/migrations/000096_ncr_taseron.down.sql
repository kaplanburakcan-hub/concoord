-- NCR taşeron bağı — geri alma
DROP TRIGGER IF EXISTS trg_ncr_subcontractor_project ON ncrs;
DROP FUNCTION IF EXISTS check_ncr_subcontractor_project();
DROP INDEX IF EXISTS idx_ncrs_subcontractor;
ALTER TABLE ncrs DROP COLUMN IF EXISTS subcontractor_id;
