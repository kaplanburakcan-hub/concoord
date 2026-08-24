-- Tedarik Planı — Satın Alma altında, mevcut PR/PO akışına dokunmayan,
-- zorunlu olmayan bir planlama/takip referansı. Excel'den toplu içe
-- aktarılabilir (bkz. internal/procurement/plan.go, payments/import.go'daki
-- harici kütüphanesiz .xlsx/.csv okuma deseniyle aynı — ADR-0003).
CREATE TABLE procurement_plan_items (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    poz_no                text NULL,
    description           text NOT NULL,
    category              text NULL,
    quantity              numeric NULL,
    unit                  text NULL,
    supplier_name         text NULL,
    planned_order_date    date NULL,
    planned_delivery_date date NULL,
    criticality           text NULL CHECK (criticality IS NULL OR criticality IN ('Kritik', 'Normal')),
    status                text NOT NULL DEFAULT 'Planlandi'
        CHECK (status IN ('Planlandi', 'SiparisVerildi', 'Yolda', 'TeslimAlindi', 'Gecikti')),
    note                  text NULL,
    created_by            uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz NULL,
    row_version           integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_procurement_plan_items_project ON procurement_plan_items (project_id)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_procurement_plan_items_updated_at BEFORE UPDATE ON procurement_plan_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
