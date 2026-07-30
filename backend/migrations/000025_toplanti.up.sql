-- Faz 25 — Toplantı Tutanakları (PMP Communications Management)
CREATE TABLE IF NOT EXISTS project_meetings (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id              UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    toplanti_no             TEXT,
    baslik                  TEXT        NOT NULL,
    toplanti_turu           TEXT        NOT NULL DEFAULT 'ilerleme'
                                        CHECK (toplanti_turu IN
                                            ('kickoff','ilerleme','saha','taseron','risk','teknik','acil')),
    tarih                   DATE        NOT NULL,
    baslangic_saati         TIME,
    bitis_saati             TIME,
    lokasyon                TEXT,
    katilimcilar            TEXT[]      NOT NULL DEFAULT '{}',
    gundem                  TEXT,
    kararlar                TEXT,
    aksiyon_maddeleri       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    sonraki_toplanti_tarihi DATE,
    durum                   TEXT        NOT NULL DEFAULT 'planli'
                                        CHECK (durum IN ('planli','gerceklesti','iptal')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pm_project_tarih ON project_meetings(project_id, tarih DESC);
