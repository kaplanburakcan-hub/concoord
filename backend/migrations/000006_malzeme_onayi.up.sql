-- Faz 5 / Migration 6 — Malzeme Onay Süreci (Submittals / MAR)
-- Plan §6.5, §8 (Faz 5). MAR yaşam döngüsü:
--   Submitted → UnderReview → Approved | ConditionallyApproved | Rejected
-- Karar notu (decision_note) karar anında ZORUNLUDUR (uygulama katmanında
-- doğrulanır; DB'de karar verilmişse boş olamaz kısıtı aşağıda).
-- Doküman ekleri Faz 2 polimorfik motoru üzerinden bağlanır:
--   documents.entity_type = 'material_approval', entity_id = MAR id.

CREATE TABLE material_approvals (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    subcontractor_id  uuid NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    mar_no            text NOT NULL,             -- proje içinde tekil: MAR-001, MAR-002...
    material_name     text NOT NULL,
    spec_ref          text NULL,                 -- şartname/teknik föy referansı
    manufacturer      text NULL,
    status            text NOT NULL DEFAULT 'Submitted'
                      CHECK (status IN ('Submitted','UnderReview','Approved',
                                        'ConditionallyApproved','Rejected')),
    decision_note     text NULL,
    decided_by        uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    decided_at        timestamptz NULL,
    created_by        uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz NULL,
    row_version       integer NOT NULL DEFAULT 1,

    -- Karar verilmiş bir MAR'da karar notu boş olamaz (karar notu zorunluluğu).
    CONSTRAINT chk_mar_decision_note CHECK (
        status IN ('Submitted','UnderReview')
        OR (decision_note IS NOT NULL AND length(btrim(decision_note)) > 0)
    )
);

-- Proje içinde MAR numarası tekil (soft-delete edilenler numarayı serbest bırakmaz:
-- kayıt defteri bütünlüğü için silinenler de numara tutar).
CREATE UNIQUE INDEX uq_material_approvals_no ON material_approvals (project_id, mar_no);
CREATE INDEX idx_mar_project_status ON material_approvals (project_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_mar_subcontractor  ON material_approvals (subcontractor_id)  WHERE deleted_at IS NULL;

CREATE TRIGGER trg_material_approvals_updated_at BEFORE UPDATE ON material_approvals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
