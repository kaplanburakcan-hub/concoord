-- Faz 3.2 (E3) — geri alma
DROP TABLE IF EXISTS project_decisions;
DROP FUNCTION IF EXISTS check_decision_supersedes_project();
DROP FUNCTION IF EXISTS forbid_decision_mutation();
