-- Migration 000062 — İş Programı (WBS + Gantt), Blok 2 Aşama 1. Fiziksel
-- ilerleme elle girilmez, onaylı (Finalized) hakedişten türetilir —
-- bu hesap burada DEĞİL, istek anında çalışan bir Go servis fonksiyonunda
-- yapılır (bkz. internal/schedule/progress.go); bu yüzden burada bir
-- materialized view veya cache tablosu YOKTUR.

CREATE TABLE schedule_items (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid          NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id       uuid          REFERENCES schedule_items(id) ON DELETE CASCADE,
    wbs_code        text          NOT NULL,
    name            text          NOT NULL,
    sort_order      integer       NOT NULL DEFAULT 0,
    is_milestone    boolean       NOT NULL DEFAULT false,

    baseline_start  date,
    baseline_finish date,
    actual_start    date,
    actual_finish   date,

    -- İlerlemenin nereden geleceği: 'derived' ise bağlı pozların onaylı
    -- hakediş kümülatifinden hesaplanır, 'manual' ise manual_progress kullanılır.
    progress_source text          NOT NULL DEFAULT 'manual' CHECK (progress_source IN ('manual','derived')),
    manual_progress numeric(5,2)  CHECK (manual_progress IS NULL OR (manual_progress >= 0 AND manual_progress <= 100)),
    weight          numeric(12,4) NOT NULL DEFAULT 0,

    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    row_version     integer       NOT NULL DEFAULT 1,
    UNIQUE (project_id, wbs_code)
);
CREATE INDEX idx_schedule_items_project ON schedule_items (project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedule_items_parent  ON schedule_items (parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_schedule_items_updated_at BEFORE UPDATE ON schedule_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Bir WBS kalemi bir veya daha fazla sözleşme pozuna (work_items) bağlanabilir.
-- DÜZELTME (Aşama 0, bkz. docs/blok2-spec.md): poz_id, work_items(id)'e FK'lıdır
-- — şartnamenin orijinal taslağında "mevcut poz/kalem tablosu" olarak
-- belirsiz bırakılmıştı.
CREATE TABLE schedule_item_pozlar (
    schedule_item_id uuid NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
    poz_id           uuid NOT NULL REFERENCES work_items(id) ON DELETE RESTRICT,
    PRIMARY KEY (schedule_item_id, poz_id)
);
CREATE INDEX idx_sip_poz ON schedule_item_pozlar (poz_id);

CREATE TABLE schedule_dependencies (
    predecessor_id uuid    NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
    successor_id   uuid    NOT NULL REFERENCES schedule_items(id) ON DELETE CASCADE,
    dep_type       text    NOT NULL DEFAULT 'FS' CHECK (dep_type IN ('FS','SS','FF','SF')),
    lag_days       integer NOT NULL DEFAULT 0,
    PRIMARY KEY (predecessor_id, successor_id),
    CHECK (predecessor_id <> successor_id)
);
CREATE INDEX idx_sd_successor ON schedule_dependencies (successor_id);

-- Baseline dondurma: idareye verilen programın revizyon geçmişi. Dondurulmuş
-- bir revizyon HİÇBİR ZAMAN güncellenmez/silinmez (yalnızca INSERT) —
-- snapshot o andaki tüm schedule_items ağacının JSON kopyasıdır.
CREATE TABLE schedule_baselines (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision_no integer     NOT NULL,
    frozen_at   timestamptz NOT NULL DEFAULT now(),
    frozen_by   uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    note        text,
    snapshot    jsonb       NOT NULL,
    UNIQUE (project_id, revision_no)
);
CREATE INDEX idx_schedule_baselines_project ON schedule_baselines (project_id);
