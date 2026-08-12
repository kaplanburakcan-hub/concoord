-- Migration 000037 — Tedarikçiler (backend'e taşıma)
--
-- Önceden tamamen tarayıcı localStorage'ında tutulan bir "Tedarikçi" kaydı
-- vardı (SubcontractorsPage.tsx'teki TedarikciPanel + TaseronDashboardPage.tsx
-- KPI/tablosu) — hiçbir kullanıcı/cihaz arasında paylaşılmıyordu. Bu tablo,
-- subcontractors ile birebir aynı şekilde, ama ayrı bir varlık olarak
-- tanımlanır: subcontractors 9 farklı modüle FK ile bağlı olduğundan
-- Tedarikçi'yi oraya bir "kind" ayrımıyla karıştırmak yerine ayrı tablo
-- tercih edildi (bkz. plan notu).

CREATE TABLE tedarikciler (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    company_name   text NOT NULL,
    tax_no         text NULL,
    contact_person text NULL,
    phone          text NULL,
    email          text NULL,
    trade          text NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_tedarikciler_project ON tedarikciler (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_tedarikciler_updated_at BEFORE UPDATE ON tedarikciler
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
