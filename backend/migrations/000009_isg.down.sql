-- Faz 8 geri alma — bağımlılık sırasıyla.
DROP TRIGGER IF EXISTS trg_ohs_pen_lock ON ohs_penalties;
DROP FUNCTION IF EXISTS lock_ohs_penalty();
DROP TRIGGER IF EXISTS trg_ohs_pen_updated_at ON ohs_penalties;
DROP TABLE IF EXISTS ohs_penalties;

DROP TRIGGER IF EXISTS trg_ohs_find_lock ON ohs_findings;
DROP FUNCTION IF EXISTS lock_closed_finding();
DROP TRIGGER IF EXISTS trg_ohs_find_updated_at ON ohs_findings;
DROP TABLE IF EXISTS ohs_findings;

DROP TRIGGER IF EXISTS trg_ohs_insp_lock ON ohs_inspections;
DROP FUNCTION IF EXISTS lock_ohs_inspection();
DROP TRIGGER IF EXISTS trg_ohs_insp_updated_at ON ohs_inspections;
DROP TABLE IF EXISTS ohs_inspections;

DROP TRIGGER IF EXISTS trg_ohs_tmpl_updated_at ON ohs_checklist_templates;
DROP TABLE IF EXISTS ohs_checklist_templates;
