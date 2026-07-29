CREATE TABLE IF NOT EXISTS project_personnel (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ad_soyad   TEXT        NOT NULL,
    gorev      TEXT        NOT NULL DEFAULT 'İşçi',
    firma      TEXT,
    is_aktif   BOOLEAN     NOT NULL DEFAULT true,
    sira       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pp_project ON project_personnel(project_id);

CREATE TABLE IF NOT EXISTS project_puantaj (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID           NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    personel_id UUID           NOT NULL REFERENCES project_personnel(id) ON DELETE CASCADE,
    tarih       DATE           NOT NULL,
    durum       TEXT           NOT NULL DEFAULT 'mevcut',
    mesai_saat  NUMERIC(4,1)   NOT NULL DEFAULT 0,
    not_metin   TEXT,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE (personel_id, tarih),
    CONSTRAINT project_puantaj_durum_check
        CHECK (durum IN ('mevcut','devamsiz','izinli','raporlu'))
);
CREATE INDEX IF NOT EXISTS idx_ppq_project_tarih ON project_puantaj(project_id, tarih);
