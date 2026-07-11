-- Faz 9 / Migration 10 — Dashboard, EVM ve yönetim raporlaması (Plan §7, Faz 9)
--
--  * pv_plan_entries          : PV S-eğrisinin aylık planlanan dağılımı (%).
--                               Girilmezse motor milestone ağırlıklarından,
--                               o da yoksa doğrusal dağılımdan türetir.
--  * project_control_settings : eşik tabanlı uyarı parametreleri (CPI/SPI alt
--                               eşiği, bulgu yaşlanma günü). Kontrol tanımları
--                               VERİ olarak tutulur (Plan §7 genişletilebilirlik).
--  * control_alerts           : üretilen uyarıların tekilleştirme defteri —
--                               aynı uyarı aynı dönemde bir kez bildirilir.
--                               APPEND-ONLY (bu yüzden std. kolon seti yok).
--  * monthly_reports          : aylık yönetim raporu (EVM + finansal + İSG +
--                               tedarik); weekly_reports ile aynı desen:
--                               snapshot API'de dondurulur, PDF worker'da üretilir.

-- ---------------------------------------------------------------------------
-- PV aylık dağılım girişi (Plan §7 EVM: "Faz 9'da aylık dağılım girişiyle
-- zenginleşir"). month daima ayın 1'i; planned_pct dönemin (kümülatif değil)
-- planlanan yüzdesidir. Toplamın 100 olması handler'da doğrulanır (±0.5 tolerans).
-- ---------------------------------------------------------------------------
CREATE TABLE pv_plan_entries (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    month       date NOT NULL CHECK (date_trunc('month', month) = month),
    planned_pct numeric(7,3) NOT NULL CHECK (planned_pct >= 0 AND planned_pct <= 100),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL,
    row_version integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_pv_plan_month ON pv_plan_entries (project_id, month)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_pv_plan_updated_at BEFORE UPDATE ON pv_plan_entries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Proje kontrol eşikleri (yeni kontrol = konfigürasyon, kod değil)
-- ---------------------------------------------------------------------------
CREATE TABLE project_control_settings (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    cpi_min            numeric(4,2) NOT NULL DEFAULT 0.90 CHECK (cpi_min > 0 AND cpi_min <= 2),
    spi_min            numeric(4,2) NOT NULL DEFAULT 0.90 CHECK (spi_min > 0 AND spi_min <= 2),
    finding_aging_days integer      NOT NULL DEFAULT 14   CHECK (finding_aging_days >= 1),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz NULL,
    row_version        integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_control_settings_project ON project_control_settings (project_id)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_control_settings_updated_at BEFORE UPDATE ON project_control_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Uyarı tekilleştirme defteri: (proje, uyarı anahtarı, dönem) tekil.
-- alert_key örnekleri: 'cpi_low', 'spi_low', 'milestone_late:<id>',
-- 'finding_aging:<id>'. period 'YYYY-MM' — aynı ay içinde tekrar bildirim yok.
-- ---------------------------------------------------------------------------
CREATE TABLE control_alerts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    alert_key  text NOT NULL,
    period     text NOT NULL,
    detail     text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, alert_key, period)
);
CREATE INDEX idx_control_alerts_project ON control_alerts (project_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- Aylık yönetim raporu (weekly_reports deseni; Plan Faz 9 kabul kriteri:
-- "aylık rapor tek tıkla üretiliyor")
-- ---------------------------------------------------------------------------
CREATE TABLE monthly_reports (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    year                  integer NOT NULL,
    month                 integer NOT NULL CHECK (month BETWEEN 1 AND 12),
    period_start          date NOT NULL,
    period_end            date NOT NULL,
    status                text NOT NULL DEFAULT 'Pending'
                          CHECK (status IN ('Pending','Ready','Failed')),
    snapshot              jsonb NOT NULL,
    generated_pdf_file_id uuid NULL REFERENCES files (id) ON DELETE RESTRICT,
    generated_by          uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    error                 text NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz NULL,
    row_version           integer NOT NULL DEFAULT 1,

    CONSTRAINT chk_mr_period CHECK (period_end >= period_start)
);
CREATE INDEX idx_monthly_reports_project
    ON monthly_reports (project_id, period_start DESC) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_monthly_reports_updated_at BEFORE UPDATE ON monthly_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
