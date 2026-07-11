-- Faz 1 / Migration 2 — Kimlik ve Yetki (RBAC + kullanıcı bazlı override)
-- Plan §4, §6.1. İlke: yetki koda gömülmez, veriye yazılır.

CREATE EXTENSION IF NOT EXISTS "citext";  -- e-posta büyük/küçük harf duyarsız

-- ---------------------------------------------------------------------------
-- Kullanıcılar
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL,
    username      text   NOT NULL,
    password_hash text   NOT NULL,
    full_name     text   NOT NULL,
    phone         text   NULL,
    is_active     boolean NOT NULL DEFAULT true,
    last_login_at timestamptz NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,
    row_version   integer NOT NULL DEFAULT 1
);
-- Soft delete ile uyumlu benzersizlik: yalnızca yaşayan kayıtlarda tekil.
CREATE UNIQUE INDEX uq_users_email    ON users (email)    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Faz 0'da forward-declare edilen audit aktör FK'si artık bağlanabilir.
ALTER TABLE audit_logs
    ADD CONSTRAINT fk_audit_logs_actor FOREIGN KEY (actor_id)
    REFERENCES users (id) ON DELETE SET NULL;
ALTER TABLE workflow_transitions
    ADD CONSTRAINT fk_wf_transitions_actor FOREIGN KEY (actor_id)
    REFERENCES users (id) ON DELETE SET NULL;

-- ---------------------------------------------------------------------------
-- Roller ve izin sözlüğü (referans veri — seed ile doldurulur)
-- ---------------------------------------------------------------------------
CREATE TABLE roles (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text NOT NULL UNIQUE,   -- Admin, ProjectManager, ...
    name       text NOT NULL,
    is_system  boolean NOT NULL DEFAULT true, -- sistem rolleri silinemez
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        text NOT NULL UNIQUE,  -- 'progress_payments.finalize'
    module      text NOT NULL,
    action      text NOT NULL,
    description text NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (module, action)
);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles (id)       ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ---------------------------------------------------------------------------
-- Projeler — Faz 2'nin CRUD/milestone/doküman katmanı bu tabloyu genişletir.
-- Faz 1'de proje bazlı yetki ve üyelik için gereken çekirdek tanım. (Plan §6.2)
-- ---------------------------------------------------------------------------
CREATE TABLE projects (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code         text NOT NULL,
    name         text NOT NULL,
    location     text NULL,
    client_name  text NULL,
    budget_total numeric(18,2) NULL,
    currency     text NOT NULL DEFAULT 'TRY',
    start_date   date NULL,
    end_date     date NULL,
    status       text NOT NULL DEFAULT 'Planning'
                 CHECK (status IN ('Planning','Active','OnHold','Closed','Archived')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_projects_code ON projects (code) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Proje üyeliği (rol proje bazlıdır: kullanıcı bir projede SiteEngineer,
-- diğerinde Client olabilir). subcontractor_id FK'si Faz 3'te bağlanır.
-- ---------------------------------------------------------------------------
CREATE TABLE project_members (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    user_id          uuid NOT NULL REFERENCES users (id)    ON DELETE RESTRICT,
    role_id          uuid NOT NULL REFERENCES roles (id)    ON DELETE RESTRICT,
    subcontractor_id uuid NULL,   -- Faz 3: subcontractors(id) FK
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_project_members ON project_members (project_id, user_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_project_members_user ON project_members (user_id);
CREATE TRIGGER trg_project_members_updated_at BEFORE UPDATE ON project_members
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Kullanıcı bazlı izin override (Plan §4 — karar önceliği: DENY > GRANT > rol)
-- project_id NULL = global override (proje bağımsız, örn. admin.* ekranları).
-- ---------------------------------------------------------------------------
CREATE TABLE user_permissions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users (id)       ON DELETE CASCADE,
    project_id    uuid NULL     REFERENCES projects (id)    ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    effect        text NOT NULL CHECK (effect IN ('GRANT','DENY')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    row_version   integer NOT NULL DEFAULT 1
);
-- Bir (kullanıcı, proje, izin) üçlüsü için tek override. NULL project_id'ler
-- eşit sayılır (PG16 NULLS NOT DISTINCT) — global override tekilliği için.
CREATE UNIQUE INDEX uq_user_permissions
    ON user_permissions (user_id, project_id, permission_id) NULLS NOT DISTINCT;
CREATE INDEX idx_user_permissions_lookup ON user_permissions (user_id, permission_id);
CREATE TRIGGER trg_user_permissions_updated_at BEFORE UPDATE ON user_permissions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Refresh token deposu (iptal edilebilirlik için; access token stateless kalır)
-- ---------------------------------------------------------------------------
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,   -- SHA-256(token); ham token asla saklanmaz
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    user_agent text NULL,
    ip         text NULL
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- Şifre sıfırlama jetonları (e-posta gönderimi Faz 4 bildirim motoruna bağlanır;
-- Faz 1'de jeton üretilir ve loglanır — sözleşme hazır)
-- ---------------------------------------------------------------------------
CREATE TABLE password_reset_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_pwreset_user ON password_reset_tokens (user_id) WHERE used_at IS NULL;
