-- Faz 6 / Migration 7 — Günlük/Haftalık Saha Raporlama
-- Plan §6.6, §8 (Faz 6).
--
-- Günlük rapor yaşam döngüsü: Draft → Submitted.
-- Geçmiş gün düzeltmesi = REVİZYON (Plan Faz 6): Submitted rapor değiştirilemez
-- (DB trigger kilidi, hakedişteki Finalized kilidiyle aynı desen); düzeltme,
-- parent_report_id referanslı yeni bir revizyon (revision_no+1) olarak açılır.
-- Bir (proje, tarih) için "geçerli" rapor = en yüksek revizyon.
--
-- Haftalık rapor: worker tarafından üretilen PDF + snapshot JSONB (veri
-- dondurma — Plan Faz 6 kabul kriteri: "PDF rakamları snapshot'tan
-- doğrulanabiliyor"). Snapshot üretim anında API'de derlenir ve bir daha
-- değişmez; PDF yalnızca snapshot'tan çizilir.

-- ---------------------------------------------------------------------------
-- Günlük rapor başlığı
-- ---------------------------------------------------------------------------
CREATE TABLE daily_reports (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    report_date      date NOT NULL,
    revision_no      integer NOT NULL DEFAULT 1,
    parent_report_id uuid NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    weather          jsonb NULL,          -- {condition, wind_kph, precipitation_mm, source}
    temperature_min  numeric(5,1) NULL,
    temperature_max  numeric(5,1) NULL,
    notes            text NULL,
    status           text NOT NULL DEFAULT 'Draft'
                     CHECK (status IN ('Draft','Submitted')),
    author_id        uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    submitted_at     timestamptz NULL,
    submitted_by     uuid NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1,

    CONSTRAINT chk_dr_revision CHECK (revision_no >= 1),
    -- Revizyon > 1 ise mutlaka bir önceki rapora bağlıdır.
    CONSTRAINT chk_dr_parent CHECK (
        (revision_no = 1 AND parent_report_id IS NULL)
        OR (revision_no > 1 AND parent_report_id IS NOT NULL)
    )
);

-- Plan §6.6: UNIQUE(project, date) — revizyon desteğiyle genişletildi.
-- Soft-delete edilen taslaklar tarihi serbest bırakır (partial index).
CREATE UNIQUE INDEX uq_daily_reports_date_rev
    ON daily_reports (project_id, report_date, revision_no)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_daily_reports_project_date
    ON daily_reports (project_id, report_date DESC) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_daily_reports_updated_at BEFORE UPDATE ON daily_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Günlük rapor satırları (personel / ekipman / imalat)
-- ---------------------------------------------------------------------------
CREATE TABLE daily_manpower (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id  uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    subcontractor_id uuid NULL REFERENCES subcontractors (id) ON DELETE RESTRICT,
    trade            text NOT NULL,       -- meslek/branş: kalıpçı, demirci...
    headcount        integer NOT NULL CHECK (headcount >= 0),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz NULL,
    row_version      integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_manpower_report ON daily_manpower (daily_report_id);

CREATE TABLE daily_equipment (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    equipment_name  text NOT NULL,
    count           integer NOT NULL CHECK (count >= 0),
    working_hours   numeric(6,1) NULL CHECK (working_hours IS NULL OR working_hours >= 0),
    idle_reason     text NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    row_version     integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_equipment_report ON daily_equipment (daily_report_id);

CREATE TABLE daily_work_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    work_item_id    uuid NULL REFERENCES work_items (id) ON DELETE RESTRICT,
    location        text NULL,           -- blok/kat/aks
    description     text NOT NULL,
    qty             numeric(14,3) NULL CHECK (qty IS NULL OR qty >= 0),
    unit            text NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    row_version     integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_work_entries_report ON daily_work_entries (daily_report_id);
CREATE INDEX idx_daily_work_entries_item   ON daily_work_entries (work_item_id)
    WHERE work_item_id IS NOT NULL;

CREATE TRIGGER trg_daily_manpower_updated_at BEFORE UPDATE ON daily_manpower
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_daily_equipment_updated_at BEFORE UPDATE ON daily_equipment
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_daily_work_entries_updated_at BEFORE UPDATE ON daily_work_entries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Değişmezlik kilidi: Submitted rapor ve satırları değiştirilemez
-- (hakedişteki lock_finalized_payment deseniyle aynı; düzeltme = revizyon).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION lock_submitted_daily_report() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.status = 'Submitted' THEN
            RAISE EXCEPTION 'Gönderilmiş günlük rapor silinemez (id=%)', OLD.id
                USING ERRCODE = 'restrict_violation';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.status = 'Submitted' THEN
        RAISE EXCEPTION 'Gönderilmiş günlük rapor değiştirilemez; düzeltme yeni revizyonla yapılır (id=%)', OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dr_lock BEFORE UPDATE OR DELETE ON daily_reports
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_report();

CREATE OR REPLACE FUNCTION lock_submitted_daily_children() RETURNS trigger AS $$
DECLARE
    rep_id uuid;
    rep_st text;
BEGIN
    rep_id := COALESCE(NEW.daily_report_id, OLD.daily_report_id);
    SELECT status INTO rep_st FROM daily_reports WHERE id = rep_id;
    IF rep_st = 'Submitted' THEN
        RAISE EXCEPTION 'Gönderilmiş günlük raporun satırları değiştirilemez (rapor=%)', rep_id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dm_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_manpower
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();
CREATE TRIGGER trg_de_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_equipment
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();
CREATE TRIGGER trg_dw_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_work_entries
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();

-- ---------------------------------------------------------------------------
-- Haftalık rapor (worker üretimi + snapshot dondurma)
-- ---------------------------------------------------------------------------
CREATE TABLE weekly_reports (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    week_no               integer NOT NULL,       -- ISO hafta numarası
    period_start          date NOT NULL,          -- Pazartesi
    period_end            date NOT NULL,          -- Pazar
    status                text NOT NULL DEFAULT 'Pending'
                          CHECK (status IN ('Pending','Ready','Failed')),
    snapshot              jsonb NOT NULL,         -- üretim anında dondurulan veri
    generated_pdf_file_id uuid NULL REFERENCES files (id) ON DELETE RESTRICT,
    generated_by          uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    error                 text NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz NULL,
    row_version           integer NOT NULL DEFAULT 1,

    CONSTRAINT chk_wr_period CHECK (period_end >= period_start)
);
CREATE INDEX idx_weekly_reports_project
    ON weekly_reports (project_id, period_start DESC) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_weekly_reports_updated_at BEFORE UPDATE ON weekly_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
