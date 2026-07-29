CREATE TABLE IF NOT EXISTS project_design_docs (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    disiplin   TEXT        NOT NULL,
    poz_no     TEXT,
    baslik     TEXT        NOT NULL,
    rev_no     TEXT        NOT NULL DEFAULT '0',
    tarih      DATE,
    durum      TEXT        NOT NULL DEFAULT 'taslak',
    aciklama   TEXT,
    sira       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_design_docs_durum_check
        CHECK (durum IN ('taslak','incelemede','onaylı','revizyon_gerekli','iptal'))
);

CREATE INDEX IF NOT EXISTS idx_pdd_project ON project_design_docs(project_id);
