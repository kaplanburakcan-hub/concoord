-- Migration 000041 — Nakit Akış sistemi, Faz A: ödeme şekli temelleri
--
-- Nakit Akış sisteminin ilk fazı. Merkezi nakit hareketi defteri
-- (cash_events) kuruluyor — her para hareketi üreten modül (hakediş ödeme
-- planı, tedarikçi ödeme planı, PO ödeme planı, kasa fişi, idari hakediş
-- tahsilatı, sonraki fazlarda eklenecek) buraya bir satır yazar. Desen,
-- mevcut payment_deductions.source_entity/source_id ile aynı (migration
-- 000004) — yeni bir kalıp icat edilmiyor, genelleniyor.
--
-- Sözleşme ve tedarikçi kayıtlarına "varsayılan ödeme şekli" eklenir —
-- sonraki fazlarda PO/hakediş/ekstre ödeme planları bu varsayılanı
-- kullanacak; kullanıcı değiştirirse onay akışına düşecek (Faz C).

ALTER TABLE contracts ADD COLUMN default_payment_method text NULL
    CHECK (default_payment_method IN ('nakit','havale','cek'));
ALTER TABLE tedarikciler ADD COLUMN default_payment_method text NULL
    CHECK (default_payment_method IN ('nakit','havale','cek'));

CREATE TABLE cash_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    direction      text NOT NULL CHECK (direction IN ('in','out')),
    source_entity  text NOT NULL,
    source_id      uuid NOT NULL,
    description    text NOT NULL,
    amount         numeric(18,2) NOT NULL,
    payment_method text NULL CHECK (payment_method IN ('nakit','havale','cek')),
    event_date     date NOT NULL,
    created_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_cash_events_project_date ON cash_events (project_id, event_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_cash_events_source ON cash_events (source_entity, source_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_cash_events_updated_at BEFORE UPDATE ON cash_events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
