-- Faz 19 — Proje Keşfi (İmalat Kalemleri)
-- Proje bazlı keşif kalemleri; kategori + sıra numarası ile gruplama destekli.

CREATE TABLE project_survey_items (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    kategori       TEXT         NOT NULL,     -- Mimari, Betonarme, Cephe, Çatı, Mekanik, Elektrik, Peyzaj...
    poz_no         TEXT,                       -- Birim fiyat tarife poz numarası (opsiyonel)
    tanim          TEXT         NOT NULL,
    birim          TEXT         NOT NULL DEFAULT 'adet',
    miktar         NUMERIC(15,3) NOT NULL DEFAULT 0,
    birim_fiyat    NUMERIC(20,2) NOT NULL DEFAULT 0,
    para_birimi    TEXT         NOT NULL DEFAULT 'TRY',
    aciklama       TEXT,
    sira           INTEGER      NOT NULL DEFAULT 0,

    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_psi_project   ON project_survey_items(project_id);
CREATE INDEX idx_psi_kategori  ON project_survey_items(project_id, kategori);
