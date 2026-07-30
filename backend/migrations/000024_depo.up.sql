-- Faz 24 — Saha Deposu (Malzeme Stok ve Hareket Kayıtları)
CREATE TABLE IF NOT EXISTS site_warehouse_items (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    malzeme_adi TEXT         NOT NULL,
    kategori    TEXT         NOT NULL DEFAULT 'Genel',
    birim       TEXT         NOT NULL DEFAULT 'adet',
    mevcut_miktar NUMERIC(15,3) NOT NULL DEFAULT 0,
    min_stok    NUMERIC(15,3) NOT NULL DEFAULT 0,
    aciklama    TEXT,
    sira        INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_swi_project ON site_warehouse_items(project_id);

CREATE TABLE IF NOT EXISTS site_warehouse_movements (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    item_id     UUID         NOT NULL REFERENCES site_warehouse_items(id) ON DELETE CASCADE,
    hareket_turu TEXT        NOT NULL DEFAULT 'giris'
                             CHECK (hareket_turu IN ('giris','cikis','sayim','iade')),
    miktar      NUMERIC(15,3) NOT NULL,
    tarih       DATE         NOT NULL DEFAULT CURRENT_DATE,
    belge_no    TEXT,
    aciklama    TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT swm_miktar_check CHECK (miktar > 0)
);
CREATE INDEX IF NOT EXISTS idx_swm_project ON site_warehouse_movements(project_id, tarih DESC);
CREATE INDEX IF NOT EXISTS idx_swm_item    ON site_warehouse_movements(item_id);
