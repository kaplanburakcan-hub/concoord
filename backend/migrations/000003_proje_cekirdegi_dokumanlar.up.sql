-- Faz 2 / Migration 3 — Proje çekirdeği (milestone) + Doküman altyapısı
-- Plan §6.2, §5.2. projects/project_members tablosu Faz 1'de kuruldu; burada
-- milestone yönetimi ve doküman motoru (klasör ağacı + versiyonlama) eklenir.
--
-- İmmutability ilkesi (Plan §5.1): files ve document_versions APPEND-ONLY'dir —
-- bir versiyon yayımlandıktan sonra güncellenmez; düzeltme = yeni versiyon.
-- Bu nedenle bu iki tabloda updated_at/deleted_at/row_version bilinçli olarak yok.

-- ---------------------------------------------------------------------------
-- Milestone'lar (proje künyesinin zaman/kapsam çıpaları — EVM PV'sinin temeli)
-- ---------------------------------------------------------------------------
CREATE TABLE milestones (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    name         text NOT NULL,
    planned_date date NULL,
    actual_date  date NULL,
    weight_pct   numeric(5,2) NULL CHECK (weight_pct IS NULL OR (weight_pct >= 0 AND weight_pct <= 100)),
    status       text NOT NULL DEFAULT 'Planned'
                 CHECK (status IN ('Planned','InProgress','Completed','Delayed')),
    sort_order   integer NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_milestones_project ON milestones (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_milestones_updated_at BEFORE UPDATE ON milestones
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Fiziksel dosya kayıtları (MinIO nesne referansı). APPEND-ONLY / immutable.
-- storage_key deseni: project/{id}/documents/{doc_id}/v{n}/{filename} (Plan §5.2)
-- ---------------------------------------------------------------------------
CREATE TABLE files (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    storage_key  text NOT NULL UNIQUE,
    original_name text NOT NULL,
    mime         text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   bigint NOT NULL DEFAULT 0,
    sha256       text NOT NULL,
    uploaded_by  uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Klasör ağacı (proje kapsamlı; kendine referanslı hiyerarşi)
-- ---------------------------------------------------------------------------
CREATE TABLE folders (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    parent_id    uuid NULL REFERENCES folders (id) ON DELETE RESTRICT,
    name         text NOT NULL,
    module_scope text NULL,   -- örn. 'Contract', 'OHS' — modül klasörlerini işaretler
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1
);
-- Aynı üst klasör altında yaşayan aynı isim tekil (kök için parent_id NULL).
CREATE UNIQUE INDEX uq_folders_sibling_name
    ON folders (project_id, parent_id, name) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL;
CREATE INDEX idx_folders_project ON folders (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_folders_updated_at BEFORE UPDATE ON folders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Dokümanlar (mantıksal belge; N versiyon taşır). Polimorfik bağ:
-- entity_type/entity_id ile aynı motor sözleşme, hakediş, İSG tutanağı,
-- irsaliyeye hizmet eder (Plan §6.2). Faz 2 ilk tüketici: sözleşme/zeyilname.
-- ---------------------------------------------------------------------------
CREATE TABLE documents (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    folder_id    uuid NULL REFERENCES folders (id) ON DELETE RESTRICT,
    entity_type  text NULL,   -- örn. 'contract', 'progress_payment' (Faz 3+)
    entity_id    uuid NULL,
    title        text NOT NULL,
    doc_category text NOT NULL DEFAULT 'Other'
                 CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS','Other')),
    created_by   uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz NULL,
    row_version  integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_documents_project  ON documents (project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_folder   ON documents (folder_id)  WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_entity   ON documents (entity_type, entity_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_documents_updated_at BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Doküman versiyonları (v1, v2, ...). APPEND-ONLY / immutable: eski versiyon
-- daima erişilebilir. Her versiyon SHA-256, yükleyen, tarih, not taşır.
-- ---------------------------------------------------------------------------
CREATE TABLE document_versions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  uuid NOT NULL REFERENCES documents (id) ON DELETE RESTRICT,
    version_no   integer NOT NULL,
    file_id      uuid NOT NULL REFERENCES files (id) ON DELETE RESTRICT,
    uploaded_by  uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    note         text NULL,
    sha256       text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, version_no)
);
CREATE INDEX idx_document_versions_doc ON document_versions (document_id, version_no DESC);
