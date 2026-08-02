-- Faz 26 — Makine & Ekipman Envanter ve Günlük Puantaj
CREATE TABLE IF NOT EXISTS project_machines (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID          NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tip                 TEXT          NOT NULL CHECK (tip IN ('arac','is_makinesi','ekipman')),
    ad                  TEXT          NOT NULL,
    plaka               TEXT,
    marka               TEXT,
    model               TEXT,
    seri_no             TEXT,
    uretim_yili         INTEGER,
    sahiplik            TEXT          NOT NULL DEFAULT 'ozmal' CHECK (sahiplik IN ('ozmal','kiralik')),
    tedarikci           TEXT,
    gunluk_ucret        NUMERIC(15,2),
    durum               TEXT          NOT NULL DEFAULT 'aktif' CHECK (durum IN ('aktif','bakim','devre_disi')),
    son_bakim_tarihi    DATE,
    sonraki_bakim_tarihi DATE,
    aciklama            TEXT,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pm_project ON project_machines(project_id, tip);

CREATE TABLE IF NOT EXISTS project_machine_logs (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID          NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    machine_id      UUID          NOT NULL REFERENCES project_machines(id) ON DELETE CASCADE,
    tarih           DATE          NOT NULL DEFAULT CURRENT_DATE,
    calisma_miktari NUMERIC(10,2) NOT NULL DEFAULT 0,
    calisma_birimi  TEXT          NOT NULL DEFAULT 'saat' CHECK (calisma_birimi IN ('saat','km')),
    operator        TEXT,
    aciklama        TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pml_project  ON project_machine_logs(project_id, tarih DESC);
CREATE INDEX IF NOT EXISTS idx_pml_machine  ON project_machine_logs(machine_id);
