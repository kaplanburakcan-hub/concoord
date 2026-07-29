CREATE TABLE IF NOT EXISTS supplier_statements (
    id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID           NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tedarikci_adi  TEXT           NOT NULL,
    ekstre_no      TEXT           NOT NULL,
    ekstre_tarihi  DATE           NOT NULL,
    vade_tarihi    DATE,
    toplam_tutar   NUMERIC(20,2)  NOT NULL DEFAULT 0,
    odenen_tutar   NUMERIC(20,2)  NOT NULL DEFAULT 0,
    para_birimi    TEXT           NOT NULL DEFAULT 'TRY',
    odeme_durumu   TEXT           NOT NULL DEFAULT 'bekliyor',
    aciklama       TEXT,
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT supplier_statements_durum_check
        CHECK (odeme_durumu IN ('bekliyor','kismi_odendi','odendi','iptal'))
);
CREATE INDEX IF NOT EXISTS idx_ss_project ON supplier_statements(project_id);
