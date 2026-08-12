-- Migration 000042 — Nakit Akış Faz B: ödeme planları (kısmi ödeme) + PO↔Tedarikçi bağlama
--
-- Her tablo, ait olduğu ana kayda (hakediş/ekstre/sipariş) birden çok kısmi
-- ödeme satırı eklenebilmesini sağlar. Ödeme şekli sözleşme/tedarikçi
-- varsayılanından farklıysa (Faz C'de eklenecek onay akışı bağlanana kadar
-- bu fazda serbestçe girilebilir — onay zorunluluğu Faz C'de eklenir).

CREATE TABLE progress_payment_disbursements (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_payment_id  uuid NOT NULL REFERENCES progress_payments (id) ON DELETE RESTRICT,
    amount               numeric(18,2) NOT NULL,
    payment_method       text NOT NULL CHECK (payment_method IN ('nakit','havale','cek')),
    event_date           date NOT NULL,
    cek_keside_tarihi    date NULL,
    note                 text NULL,
    created_by           uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz NULL,
    row_version          integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ppd_payment ON progress_payment_disbursements (progress_payment_id) WHERE deleted_at IS NULL;

CREATE TABLE supplier_statement_payments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    statement_id  uuid NOT NULL REFERENCES supplier_statements (id) ON DELETE RESTRICT,
    amount        numeric(18,2) NOT NULL,
    payment_method text NOT NULL CHECK (payment_method IN ('nakit','havale','cek')),
    event_date    date NOT NULL,
    cek_keside_tarihi date NULL,
    note          text NULL,
    created_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,
    row_version   integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ssp_statement ON supplier_statement_payments (statement_id) WHERE deleted_at IS NULL;

CREATE TABLE purchase_order_payments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id         uuid NOT NULL REFERENCES purchase_orders (id) ON DELETE RESTRICT,
    amount        numeric(18,2) NOT NULL,
    payment_method text NOT NULL CHECK (payment_method IN ('nakit','havale','cek')),
    event_date    date NOT NULL,
    cek_keside_tarihi date NULL,
    note          text NULL,
    created_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,
    row_version   integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_pop_po ON purchase_order_payments (po_id) WHERE deleted_at IS NULL;

-- PO'lar artık (opsiyonel) gerçek bir tedarikçi kaydına bağlanabilir —
-- mevcut serbest metin supplier_name geriye dönük uyumluluk için kalır.
ALTER TABLE purchase_orders ADD COLUMN tedarikci_id uuid NULL REFERENCES tedarikciler (id) ON DELETE SET NULL;
