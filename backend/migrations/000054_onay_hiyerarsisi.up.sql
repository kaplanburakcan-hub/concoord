-- Migration 000054 — Genel/tekrar kullanılabilir çok kademeli onay hiyerarşisi
-- altyapısı. İlk kullanım: ekipman transfer talepleri (equipment_transfer_requests),
-- entity_type/entity_id ile herhangi bir varlığa bağlanabilir — ileride Sigorta,
-- Ana Sözleşme revizyonu gibi başka modüller aynı üç tabloyu, kendi entity_type
-- değerleriyle kullanabilir; kod değişikliği gerektirmez, sadece
-- approval_chain_steps'e yeni satırlar eklenir.

-- Bir entity_type için sıralı onay kademeleri. Kademedeki onaycı, o kademenin
-- rol_code'una sahip PROJE ÜYELERİdir (project_members.role_id -> roles.code).
CREATE TABLE approval_chain_steps (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type   text NOT NULL,
    step_order    integer NOT NULL CHECK (step_order >= 1),
    role_code     text NOT NULL,
    UNIQUE (entity_type, step_order)
);

-- Bir varlık örneği için açılan onay süreci — sıradaki kademe current_step'te.
CREATE TABLE approval_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type   text NOT NULL,
    entity_id     uuid NOT NULL,
    project_id    uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    current_step  integer NOT NULL DEFAULT 1,
    total_steps   integer NOT NULL,
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    created_by    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    decided_at    timestamptz NULL,
    UNIQUE (entity_type, entity_id)
);
CREATE INDEX idx_approval_requests_project_status ON approval_requests (project_id, status);

-- Her kademede verilen tek karar — herhangi biri 'rejected' ise zincir orada durur.
CREATE TABLE approval_decisions (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    approval_request_id  uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    step_order            integer NOT NULL,
    role_code              text NOT NULL,
    decided_by             uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    decision                text NOT NULL CHECK (decision IN ('approved','rejected')),
    note                     text NULL,
    decided_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (approval_request_id, step_order)
);

-- Ekipman transfer talepleri için 3 kademeli zincir: Şantiye Şefi/İnşaat
-- Müdürü -> Proje Yöneticisi -> Sistem Yöneticisi (bkz. rbac.Roles kodları).
INSERT INTO approval_chain_steps (entity_type, step_order, role_code) VALUES
    ('equipment_transfer', 1, 'SiteManager'),
    ('equipment_transfer', 2, 'ProjectManager'),
    ('equipment_transfer', 3, 'Admin');
