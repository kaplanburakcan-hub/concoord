-- Migration 000040 — Özel Raporlar (Proje İzleme Raporları'nda "özel rapor
-- ekle" özelliği)
--
-- Önceden tamamen React state'inde tutuluyordu — sayfa yenilenince kaybolan
-- bir liste (sayfanın kendi footer'ı bunu itiraf ediyordu). Diğer küçük
-- özelliklerle aynı desende gerçek bir tabloya taşınıyor.

CREATE TABLE custom_reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    label       text NOT NULL,
    description text NULL,
    created_by  uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL,
    row_version integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_custom_reports_project ON custom_reports (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_custom_reports_updated_at BEFORE UPDATE ON custom_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
