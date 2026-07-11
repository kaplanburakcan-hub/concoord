-- Faz 8 / Migration 9 — İSG Modülü
-- Plan §6.6, §8 (Faz 8).
--
-- Dört varlık: checklist şablonları (admin tanımlar, items JSONB — kontrol
-- tanımları VERİ olarak tutulur, Plan §7 genişletilebilirlik ilkesi),
-- denetimler (mobil, foto/GPS), bulgular (Open→InProgress→Closed, termin
-- takibi) ve ceza tutanakları (PDF + hakediş kesinti köprüsü).
--
-- Değişmezlik: gönderilmiş denetim ve kesilmiş ceza tutanağı KANITTIR —
-- içerik alanları DB trigger'ıyla kilitlenir (hakediş Finalized deseni,
-- SQLSTATE 23001 → API 409). Ceza üzerinde yalnızca statü ilerlemesi ve
-- otomasyon alanları (pdf_file_id, applied_payment_id) güncellenebilir.

-- ---------------------------------------------------------------------------
-- Checklist şablonları
-- ---------------------------------------------------------------------------
CREATE TABLE ohs_checklist_templates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    category    text NOT NULL DEFAULT 'Genel',      -- Yüksekte Çalışma, Elektrik, KKD...
    items       jsonb NOT NULL DEFAULT '[]',        -- [{"no":1,"text":"...","critical":false}]
    is_active   boolean NOT NULL DEFAULT true,
    created_by  uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL,
    row_version integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_ohs_template_name ON ohs_checklist_templates (name)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_ohs_tmpl_updated_at BEFORE UPDATE ON ohs_checklist_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Denetimler (mobil saha; offline kuyruktan da gelir)
-- ---------------------------------------------------------------------------
CREATE TABLE ohs_inspections (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    template_id   uuid NOT NULL REFERENCES ohs_checklist_templates (id) ON DELETE RESTRICT,
    inspector_id  uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    inspected_at  timestamptz NOT NULL DEFAULT now(),  -- offline girişte cihaz saati gönderilir
    location_text text NULL,
    gps_lat       double precision NULL,
    gps_lng       double precision NULL,
    results       jsonb NOT NULL DEFAULT '[]',   -- [{"no":1,"answer":"ok|fail|na","note":"..."}]
    score         numeric(5,2) NULL,             -- uygun/uygulanabilir %, sunucu hesaplar
    status        text NOT NULL DEFAULT 'Submitted'
                  CHECK (status IN ('Submitted')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz NULL,
    row_version   integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ohs_insp_project ON ohs_inspections (project_id, inspected_at DESC)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_ohs_insp_updated_at BEFORE UPDATE ON ohs_inspections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Gönderilmiş denetim kanıttır: içerik güncellenemez, silinemez (soft delete
-- dahil — purge yalnızca admin onaylı fiziksel süreçtir, Plan §5.1).
CREATE OR REPLACE FUNCTION lock_ohs_inspection() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'İSG denetimi silinemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    IF NEW.results     IS DISTINCT FROM OLD.results
    OR NEW.template_id IS DISTINCT FROM OLD.template_id
    OR NEW.inspector_id IS DISTINCT FROM OLD.inspector_id
    OR NEW.inspected_at IS DISTINCT FROM OLD.inspected_at
    OR NEW.deleted_at  IS DISTINCT FROM OLD.deleted_at THEN
        RAISE EXCEPTION 'Gönderilmiş İSG denetimi değiştirilemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_ohs_insp_lock BEFORE UPDATE OR DELETE ON ohs_inspections
    FOR EACH ROW EXECUTE FUNCTION lock_ohs_inspection();

-- ---------------------------------------------------------------------------
-- Bulgular (foto + lokasyon; yaşam döngüsü + termin takibi)
-- ---------------------------------------------------------------------------
CREATE TABLE ohs_findings (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    inspection_id     uuid NULL REFERENCES ohs_inspections (id) ON DELETE RESTRICT,
    subcontractor_id  uuid NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    severity          text NOT NULL
                      CHECK (severity IN ('Observation','Minor','Major','Critical')),
    description       text NOT NULL,
    location          text NULL,
    gps_lat           double precision NULL,
    gps_lng           double precision NULL,
    photo_document_id uuid NULL REFERENCES documents (id) ON DELETE RESTRICT,
    due_date          date NULL,                 -- termin; geçerse worker bildirir
    status            text NOT NULL DEFAULT 'Open'
                      CHECK (status IN ('Open','InProgress','Closed')),
    reported_by       uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    closed_by         uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    closed_at         timestamptz NULL,
    close_note        text NULL,                 -- kapatmada ZORUNLU (handler)
    overdue_notified_at timestamptz NULL,        -- termin bildirimi tekrarlanmaz
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz NULL,
    row_version       integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ohs_find_project_status ON ohs_findings (project_id, status)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_ohs_find_sub ON ohs_findings (subcontractor_id)
    WHERE deleted_at IS NULL;
-- Termin taraması: süresi geçmiş açık bulgular.
CREATE INDEX idx_ohs_find_due ON ohs_findings (due_date)
    WHERE deleted_at IS NULL AND status IN ('Open','InProgress');
CREATE TRIGGER trg_ohs_find_updated_at BEFORE UPDATE ON ohs_findings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Kapanmış bulgu yeniden açılamaz/değiştirilemez (izlenebilirlik).
CREATE OR REPLACE FUNCTION lock_closed_finding() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    IF OLD.status = 'Closed' THEN
        RAISE EXCEPTION 'Kapanmış İSG bulgusu değiştirilemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_ohs_find_lock BEFORE UPDATE ON ohs_findings
    FOR EACH ROW EXECUTE FUNCTION lock_closed_finding();

-- ---------------------------------------------------------------------------
-- Ceza tutanakları (otomasyon: PDF + bildirim + hakediş kesinti önerisi)
-- ---------------------------------------------------------------------------
CREATE TABLE ohs_penalties (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    subcontractor_id     uuid NOT NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    finding_id           uuid NULL REFERENCES ohs_findings (id) ON DELETE RESTRICT,
    penalty_no           text NOT NULL,                      -- ISG-001 (proje içinde sıralı)
    violation_type       text NOT NULL,                      -- örn. "Baretsiz çalışma"
    penalty_level        text NOT NULL CHECK (penalty_level IN ('Warning','Fine')),
    amount               numeric(14,2) NULL CHECK (amount IS NULL OR amount > 0), -- Fine için ZORUNLU (handler)
    note                 text NULL,
    evidence_document_id uuid NULL REFERENCES documents (id) ON DELETE RESTRICT,
    issued_by            uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    issued_at            timestamptz NOT NULL DEFAULT now(),
    status               text NOT NULL DEFAULT 'Issued'
                         CHECK (status IN ('Issued','Acknowledged','AppliedToPayment')),
    applied_payment_id   uuid NULL REFERENCES progress_payments (id) ON DELETE RESTRICT,
    pdf_file_id          uuid NULL REFERENCES files (id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz NULL,
    row_version          integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_ohs_penalty_no ON ohs_penalties (project_id, penalty_no)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_ohs_pen_project ON ohs_penalties (project_id, issued_at DESC)
    WHERE deleted_at IS NULL;
-- Kesinti önerisi taraması: hakedişe henüz uygulanmamış para cezaları.
CREATE INDEX idx_ohs_pen_pending ON ohs_penalties (subcontractor_id)
    WHERE deleted_at IS NULL AND status IN ('Issued','Acknowledged')
      AND penalty_level = 'Fine';
CREATE TRIGGER trg_ohs_pen_updated_at BEFORE UPDATE ON ohs_penalties
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Kesilmiş tutanak kanıttır: içerik değişmez, silinemez. Beyaz liste:
-- status ilerlemesi (Issued→Acknowledged→AppliedToPayment), applied_payment_id
-- ve pdf_file_id (otomasyon alanları) + updated_at/row_version.
CREATE OR REPLACE FUNCTION lock_ohs_penalty() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'İSG ceza tutanağı silinemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    IF NEW.violation_type       IS DISTINCT FROM OLD.violation_type
    OR NEW.penalty_level        IS DISTINCT FROM OLD.penalty_level
    OR NEW.amount               IS DISTINCT FROM OLD.amount
    OR NEW.note                 IS DISTINCT FROM OLD.note
    OR NEW.subcontractor_id     IS DISTINCT FROM OLD.subcontractor_id
    OR NEW.finding_id           IS DISTINCT FROM OLD.finding_id
    OR NEW.evidence_document_id IS DISTINCT FROM OLD.evidence_document_id
    OR NEW.issued_by            IS DISTINCT FROM OLD.issued_by
    OR NEW.issued_at            IS DISTINCT FROM OLD.issued_at
    OR NEW.penalty_no           IS DISTINCT FROM OLD.penalty_no
    OR NEW.deleted_at           IS DISTINCT FROM OLD.deleted_at THEN
        RAISE EXCEPTION 'Kesilmiş İSG ceza tutanağı değiştirilemez (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    -- Statü yalnızca ileri gider.
    IF (OLD.status = 'Acknowledged' AND NEW.status = 'Issued')
    OR (OLD.status = 'AppliedToPayment' AND NEW.status <> 'AppliedToPayment') THEN
        RAISE EXCEPTION 'İSG ceza statüsü geri alınamaz (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER trg_ohs_pen_lock BEFORE UPDATE OR DELETE ON ohs_penalties
    FOR EACH ROW EXECUTE FUNCTION lock_ohs_penalty();
