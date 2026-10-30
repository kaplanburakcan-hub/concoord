-- Depo hareketine isteğe bağlı sipariş referansı (B25). Mevcut hareketler boş kalır.
-- Sipariş aynı projeye ait olmalıdır (tetikleyici).
ALTER TABLE site_warehouse_movements
    ADD COLUMN purchase_order_id uuid NULL REFERENCES purchase_orders (id) ON DELETE RESTRICT;
CREATE INDEX idx_warehouse_movements_po ON site_warehouse_movements (purchase_order_id) WHERE purchase_order_id IS NOT NULL;
CREATE OR REPLACE FUNCTION check_warehouse_po_project() RETURNS trigger AS $$
BEGIN
    IF NEW.purchase_order_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM purchase_orders o WHERE o.id = NEW.purchase_order_id AND o.project_id = NEW.project_id
    ) THEN
        RAISE EXCEPTION 'sipariş bu projeye ait değil';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_warehouse_po_project BEFORE INSERT OR UPDATE OF purchase_order_id, project_id ON site_warehouse_movements
    FOR EACH ROW EXECUTE FUNCTION check_warehouse_po_project();
