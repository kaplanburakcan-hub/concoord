-- Faz 18 — Ana Sözleşme (İşveren-Yüklenici sözleşmesi temel bilgileri)
-- Her proje için tek bir ana sözleşme kaydı tutulur (UNIQUE project_id).

CREATE TABLE project_main_contracts (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id              UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    -- Sözleşme türü
    sozlesme_turu           TEXT        NOT NULL DEFAULT 'goturu_bedel'
                                        CHECK (sozlesme_turu IN ('birim_fiyat', 'goturu_bedel', 'karma')),

    -- Fiyat farkı
    fiyat_farki_var         BOOLEAN     NOT NULL DEFAULT FALSE,
    fiyat_farki_formulu     TEXT,

    -- Götürü / karma bölüm bedeli
    sozlesme_bedeli         NUMERIC(20,2),
    sozlesme_para_birimi    TEXT        NOT NULL DEFAULT 'TRY',

    -- Birim fiyatlı / karma: iş kalemleri (JSONB dizi)
    -- [{id, tanim, birim, miktar, birim_fiyat, para_birimi}]
    birim_fiyat_kalemleri   JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- Tarihler
    sozlesme_tarihi         DATE,
    yer_teslim_tarihi       DATE,
    is_suresi_gun           INTEGER     CHECK (is_suresi_gun > 0),
    gecici_kabul_sonrasi_gun INTEGER    CHECK (gecici_kabul_sonrasi_gun >= 0),

    -- İş artış / ekiliş oranları (%)
    max_artis_orani         NUMERIC(5,2) CHECK (max_artis_orani >= 0),
    max_eksilis_orani       NUMERIC(5,2) CHECK (max_eksilis_orani >= 0),

    -- SGK
    sgk_is_yeri_no          TEXT,

    -- PDF eki
    pdf_dosya_url           TEXT,
    pdf_dosya_adi           TEXT,

    -- Denetim
    created_by              UUID        REFERENCES users(id),
    updated_by              UUID        REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (project_id)
);

CREATE INDEX idx_pmc_project ON project_main_contracts(project_id);
