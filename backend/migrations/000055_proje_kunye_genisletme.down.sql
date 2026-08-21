ALTER TABLE projects
    DROP COLUMN IF EXISTS proje_turu,
    DROP COLUMN IF EXISTS toplam_insaat_alani_m2,
    DROP COLUMN IF EXISTS kat_blok_bilgisi;
