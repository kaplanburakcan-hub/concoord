-- Faz 3 / Migration 4 — Taşeron ve Hakediş Yönetimi (Finansal Çekirdek)
-- Plan §6.3, §6.4, §5.1. Bu faz platformun finansal kayıt sistemidir; bu yüzden
-- değişmezlik (immutability) DB seviyesinde zorlanır: KESİNLEŞMİŞ (Finalized)
-- hakedişler ve kalemleri trigger ile UPDATE/DELETE'e kapatılır — düzeltme yalnızca
-- yeni revizyon (revision_of) kaydıyla yapılır.

-- ---------------------------------------------------------------------------
-- Taşeron kartları (proje kapsamlı — aynı gerçek firma her projede ayrı kayıt)
-- ---------------------------------------------------------------------------
CREATE TABLE subcontractors (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    company_name   text NOT NULL,
    tax_no         text NULL,
    contact_person text NULL,
    phone          text NULL,
    email          text NULL,
    trade          text NULL,           -- imalat kalemi / branş (ör. Kalıp, Demir)
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_subcontractors_project ON subcontractors (project_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_subcontractors_updated_at BEFORE UPDATE ON subcontractors
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Faz 1'de forward-declare edilen project_members.subcontractor_id artık bağlanır:
-- bir kullanıcı bir projede bir taşerona bağlanınca (SubcontractorRep) satır
-- seviyesi güvenlik bu bağ üzerinden uygulanır (yalnızca kendi firması).
ALTER TABLE project_members
    ADD CONSTRAINT fk_project_members_subcontractor FOREIGN KEY (subcontractor_id)
    REFERENCES subcontractors (id) ON DELETE RESTRICT;

-- ---------------------------------------------------------------------------
-- Sözleşmeler (ana / alt / zeyilname). document_id doküman motoruna (Faz 2) bağ.
-- ---------------------------------------------------------------------------
CREATE TABLE contracts (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    subcontractor_id   uuid NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    contract_no        text NOT NULL,
    type               text NOT NULL DEFAULT 'Sub'
                       CHECK (type IN ('Main','Sub','Addendum')),
    parent_contract_id uuid NULL REFERENCES contracts (id) ON DELETE RESTRICT,
    amount             numeric(18,2) NULL,
    advance_amount     numeric(18,2) NOT NULL DEFAULT 0,   -- toplam avans (mahsup edilecek)
    retention_pct      numeric(5,2)  NOT NULL DEFAULT 3.00 -- teminat kesinti oranı (%)
                       CHECK (retention_pct >= 0 AND retention_pct <= 100),
    advance_rate_pct   numeric(5,2)  NOT NULL DEFAULT 20.00 -- her dönem brütünden avans mahsup oranı (%)
                       CHECK (advance_rate_pct >= 0 AND advance_rate_pct <= 100),
    sign_date          date NULL,
    document_id        uuid NULL REFERENCES documents (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz NULL,
    row_version        integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_contracts_no ON contracts (project_id, contract_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_contracts_project ON contracts (project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_contracts_sub     ON contracts (subcontractor_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_contracts_updated_at BEFORE UPDATE ON contracts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Birim fiyat cetveli / metraj pozları (BOQ). Hakediş kalemleri buna bağlanır.
-- ---------------------------------------------------------------------------
CREATE TABLE work_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    subcontractor_id uuid NOT NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    contract_id      uuid NULL REFERENCES contracts (id) ON DELETE RESTRICT,
    poz_no           text NOT NULL,
    description      text NOT NULL,
    unit             text NOT NULL DEFAULT 'ad',
    contract_qty     numeric(18,3) NOT NULL DEFAULT 0,
    unit_price       numeric(18,2) NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_work_items_poz ON work_items (subcontractor_id, poz_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_work_items_sub ON work_items (subcontractor_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_work_items_updated_at BEFORE UPDATE ON work_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Hakedişler (kümülatif). İş akışı: Draft → Submitted → SiteApproved → Finalized.
-- gross_* / total_deductions / net_payable hesaplanmış değerlerdir (Plan §6.4);
-- her Finalize'de yeniden hesaplanıp yazılır, sonra satır kilitlenir.
-- ---------------------------------------------------------------------------
CREATE TABLE progress_payments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    subcontractor_id uuid NOT NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    period_no        integer NOT NULL,
    period_start     date NULL,
    period_end       date NULL,
    status           text NOT NULL DEFAULT 'Draft'
                     CHECK (status IN ('Draft','Submitted','SiteApproved','Finalized','Rejected')),
    gross_cum        numeric(18,2) NOT NULL DEFAULT 0,  -- A: Σ(cum_qty × unit_price)
    gross_prev       numeric(18,2) NOT NULL DEFAULT 0,  -- B: önceki Finalized'in A değeri
    gross_this       numeric(18,2) NOT NULL DEFAULT 0,  -- C: A − B
    total_deductions numeric(18,2) NOT NULL DEFAULT 0,  -- D+E+F+G+H
    vat_pct          numeric(5,2)  NOT NULL DEFAULT 20.00,
    net_payable      numeric(18,2) NOT NULL DEFAULT 0,  -- I: C − kesintiler (KDV hariç)
    revision_of      uuid NULL REFERENCES progress_payments (id) ON DELETE RESTRICT, -- düzeltme = yeni revizyon
    submitted_at     timestamptz NULL,
    site_approved_at timestamptz NULL,
    finalized_at     timestamptz NULL,
    finalized_by     uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    created_by       uuid NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1
);
-- Aynı taşeron için dönem numarası tekil (yaşayan kayıtlarda).
CREATE UNIQUE INDEX uq_pp_period ON progress_payments (subcontractor_id, period_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_pp_project ON progress_payments (project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_pp_sub     ON progress_payments (subcontractor_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_pp_updated_at BEFORE UPDATE ON progress_payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Hakediş kalemleri (poz bazında bu dönem metrajı ve kümülatif taşıma)
CREATE TABLE progress_payment_items (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_payment_id uuid NOT NULL REFERENCES progress_payments (id) ON DELETE RESTRICT,
    work_item_id        uuid NOT NULL REFERENCES work_items (id) ON DELETE RESTRICT,
    prev_cum_qty        numeric(18,3) NOT NULL DEFAULT 0,  -- önceki Finalized dönemin kümülatifi
    this_period_qty     numeric(18,3) NOT NULL DEFAULT 0,  -- cum_qty − prev_cum_qty
    cum_qty             numeric(18,3) NOT NULL DEFAULT 0,  -- bu döneme dek kümülatif
    cum_amount          numeric(18,2) NOT NULL DEFAULT 0,  -- cum_qty × unit_price
    this_amount         numeric(18,2) NOT NULL DEFAULT 0,  -- this_period_qty × unit_price
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (progress_payment_id, work_item_id)
);
CREATE INDEX idx_ppi_pp ON progress_payment_items (progress_payment_id);

-- Kesinti kalemleri (D avans, E teminat, F İSG ceza, G vergi, H diğer).
-- source_entity/source_id: otomasyon köprüsü — İSG cezası (Faz 8) buradan izlenir.
CREATE TABLE payment_deductions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_payment_id uuid NOT NULL REFERENCES progress_payments (id) ON DELETE RESTRICT,
    type                text NOT NULL
                        CHECK (type IN ('AdvanceOffset','Retention','Tax','OHSPenalty','Other')),
    source_entity       text NULL,
    source_id           uuid NULL,
    description         text NULL,
    rate_pct            numeric(7,4) NULL,
    amount              numeric(18,2) NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_pd_pp ON payment_deductions (progress_payment_id);

-- ---------------------------------------------------------------------------
-- Finansal kilitleme (Plan §5.1): Finalized hakediş satırı ve kalemleri
-- UPDATE/DELETE'e kapalıdır. Kesinleşmeye GEÇİŞ (OLD.status <> 'Finalized')
-- serbesttir; kesinleştikten sonra her değişiklik reddedilir.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION lock_finalized_payment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'Finalized' THEN
            RAISE EXCEPTION 'Kesinleşmiş hakediş silinemez (id=%)', OLD.id
                USING ERRCODE = 'restrict_violation';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'Finalized' THEN
        RAISE EXCEPTION 'Kesinleşmiş hakediş değiştirilemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_pp_lock BEFORE UPDATE OR DELETE ON progress_payments
    FOR EACH ROW EXECUTE FUNCTION lock_finalized_payment();

CREATE OR REPLACE FUNCTION lock_finalized_children() RETURNS trigger AS $$
DECLARE
    pp_id     uuid;
    pp_status text;
BEGIN
    pp_id := COALESCE(NEW.progress_payment_id, OLD.progress_payment_id);
    SELECT status INTO pp_status FROM progress_payments WHERE id = pp_id;
    IF pp_status = 'Finalized' THEN
        RAISE EXCEPTION 'Kesinleşmiş hakediş kalemleri değiştirilemez (hakediş=%)', pp_id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ppi_lock BEFORE INSERT OR UPDATE OR DELETE ON progress_payment_items
    FOR EACH ROW EXECUTE FUNCTION lock_finalized_children();
CREATE TRIGGER trg_pd_lock BEFORE INSERT OR UPDATE OR DELETE ON payment_deductions
    FOR EACH ROW EXECUTE FUNCTION lock_finalized_children();
