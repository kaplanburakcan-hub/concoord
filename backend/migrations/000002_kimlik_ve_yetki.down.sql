-- Faz 1 / Migration 2 — geri alma (bağımlılık sırasına dikkat)

ALTER TABLE IF EXISTS workflow_transitions DROP CONSTRAINT IF EXISTS fk_wf_transitions_actor;
ALTER TABLE IF EXISTS audit_logs           DROP CONSTRAINT IF EXISTS fk_audit_logs_actor;

DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS "citext";
