-- Faz 7 / Migration 8 — Satınalma ve Tedarik Zinciri
-- Plan §6.6, §8 (Faz 7).
--
-- Zincir: PR (talep) → onay → PO (sipariş) → teslimatlar (kısmi destekli).
-- İzlenebilirlik (kabul kriteri): PO.pr_id ile PR'a, deliveries.po_id ile
-- PO'ya bağlanır; her statü geçişi workflow_transitions'a yazılır (Plan §5.1).
--
-- Değişmezlik: PR Draft'tan çıktığında kalemleri kilitlenir (DB trigger —
-- hakediş Finalized / günlük rapor Submitted kilidiyle aynı desen). Karar
-- verilmiş (Approved/Rejected/Converted) PR başlığı da kilitlenir; yalnızca
-- Converted geçişi Approved üzerinde izinlidir (trigger içinde beyaz liste).

-- ---------------------------------------------------------------------------
-- Satınalma talepleri (PR)
-- ---------------------------------------------------------------------------
CREATE TABLE purchase_requests (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    pr_no          text NOT NULL,                        -- PR-001 (proje içinde sıralı)
    requested_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    needed_by_date date NOT NULL,                        -- ihtiyaç tarihi (gecikme uyarısı buradan)
    note           text NULL,
    status         text NOT NULL DEFAULT 'Draft'
                   CHECK (status IN ('Draft','Submitted','Approved','Rejected','Converted')),
    submitted_at   timestamptz NULL,
    decided_by     uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    decided_at     timestamptz NULL,
    decision_note  text NULL,                            -- Rejected için ZORUNLU (handler)
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_pr_no ON purchase_requests (project_id, pr_no)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_pr_project_status ON purchase_requests (project_id, status)
    WHERE deleted_at IS NULL;
-- Gecikme taraması: ihtiyaç tarihi geçmiş, henüz siparişe dönmemiş talepler.
CREATE INDEX idx_pr_needed_by ON purchase_requests (needed_by_date)
    WHERE deleted_at IS NULL AND status IN ('Submitted','Approved');

CREATE TRIGGER trg_pr_updated_at BEFORE UPDATE ON purchase_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE purchase_request_items (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pr_id         uuid NOT NULL REFERENCES purchase_requests (id) ON DELETE RESTRICT,
    material_name text NOT NULL,
    spec          text NULL,
    qty           numeric(14,3) NOT NULL CHECK (qty > 0),
    unit          text NOT NULL,
    note          text NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,
    row_version   integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_pri_pr ON purchase_request_items (pr_id);

CREATE TRIGGER trg_pri_updated_at BEFORE UPDATE ON purchase_request_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Kilitler: Draft dışındaki PR'ın kalemleri değiştirilemez; karara bağlanmış
-- PR başlığında yalnızca Approved→Converted geçişine izin verilir.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION lock_decided_purchase_request() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status <> 'Draft' THEN
            RAISE EXCEPTION 'Yalnızca taslak PR silinebilir (id=%)', OLD.id
                USING ERRCODE = 'restrict_violation';
        END IF;
        RETURN OLD;
    END IF;
    -- Approved → Converted (PR→PO dönüşümü) tek izinli değişikliktir.
    IF OLD.status = 'Approved' AND NEW.status = 'Converted' THEN
        RETURN NEW;
    END IF;
    IF OLD.status IN ('Approved','Rejected','Converted') THEN
        RAISE EXCEPTION 'Karara bağlanmış PR değiştirilemez (id=%, durum=%)', OLD.id, OLD.status
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_pr_lock BEFORE UPDATE OR DELETE ON purchase_requests
    FOR EACH ROW EXECUTE FUNCTION lock_decided_purchase_request();

CREATE OR REPLACE FUNCTION lock_nondraft_pr_items() RETURNS trigger AS $$
DECLARE
    prid uuid;
    st   text;
BEGIN
    prid := CASE WHEN TG_OP = 'DELETE' THEN OLD.pr_id ELSE NEW.pr_id END;
    SELECT status INTO st FROM purchase_requests WHERE id = prid;
    IF st IS NOT NULL AND st <> 'Draft' THEN
        RAISE EXCEPTION 'Taslak olmayan PR kalemleri değiştirilemez (pr=%)', prid
            USING ERRCODE = 'restrict_violation';
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_pri_lock BEFORE INSERT OR UPDATE OR DELETE ON purchase_request_items
    FOR EACH ROW EXECUTE FUNCTION lock_nondraft_pr_items();

-- ---------------------------------------------------------------------------
-- Siparişler (PO)
-- ---------------------------------------------------------------------------
CREATE TABLE purchase_orders (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    pr_id               uuid NULL REFERENCES purchase_requests (id) ON DELETE RESTRICT,
    po_no               text NOT NULL,                   -- PO-001 (proje içinde sıralı)
    supplier_name       text NOT NULL,
    amount              numeric(14,2) NULL CHECK (amount IS NULL OR amount >= 0),
    currency            text NOT NULL DEFAULT 'TRY',
    status              text NOT NULL DEFAULT 'Ordered'
                        CHECK (status IN ('Ordered','PartiallyDelivered','Delivered','Cancelled')),
    expected_date       date NULL,                       -- beklenen teslim (gecikme vurgusu)
    note                text NULL,
    created_by          uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    overdue_notified_at timestamptz NULL,                -- worker gecikme uyarısı işareti
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz NULL,
    row_version         integer NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX uq_po_no ON purchase_orders (project_id, po_no)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_po_project_status ON purchase_orders (project_id, status)
    WHERE deleted_at IS NULL;
-- Gecikme taraması: beklenen tarihi geçmiş, kapanmamış siparişler.
CREATE INDEX idx_po_expected ON purchase_orders (expected_date)
    WHERE deleted_at IS NULL AND status IN ('Ordered','PartiallyDelivered');

CREATE TRIGGER trg_po_updated_at BEFORE UPDATE ON purchase_orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Teslimatlar (kısmi teslimat destekli; irsaliye fotoğrafı Faz 2 doküman
-- motoruna document_id ile bağlanır — Plan §6.6)
-- ---------------------------------------------------------------------------
CREATE TABLE deliveries (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id            uuid NOT NULL REFERENCES purchase_orders (id) ON DELETE RESTRICT,
    delivery_note_no text NOT NULL,                      -- irsaliye no
    delivered_at     timestamptz NOT NULL DEFAULT now(),
    received_by      uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    document_id      uuid NULL REFERENCES documents (id) ON DELETE RESTRICT,
    note             text NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_deliveries_po ON deliveries (po_id) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_deliveries_updated_at BEFORE UPDATE ON deliveries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
