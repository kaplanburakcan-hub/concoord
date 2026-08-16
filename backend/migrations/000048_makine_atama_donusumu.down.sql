-- Eski sütunları geri ekle ve company_equipment'ten backfill et.
ALTER TABLE project_machines
    ADD COLUMN tip TEXT,
    ADD COLUMN ad TEXT,
    ADD COLUMN plaka TEXT,
    ADD COLUMN marka TEXT,
    ADD COLUMN model TEXT,
    ADD COLUMN seri_no TEXT,
    ADD COLUMN uretim_yili INTEGER,
    ADD COLUMN sahiplik TEXT,
    ADD COLUMN tedarikci TEXT,
    ADD COLUMN gunluk_ucret NUMERIC(15,2),
    ADD COLUMN durum TEXT,
    ADD COLUMN son_bakim_tarihi DATE,
    ADD COLUMN sonraki_bakim_tarihi DATE,
    ADD COLUMN aciklama TEXT;

UPDATE project_machines pm SET
    tip = ce.tip, ad = ce.ad, plaka = ce.plaka, marka = ce.marka,
    model = ce.model, seri_no = ce.seri_no, uretim_yili = ce.uretim_yili,
    sahiplik = ce.sahiplik, tedarikci = ce.tedarikci, gunluk_ucret = ce.gunluk_ucret,
    durum = ce.durum, son_bakim_tarihi = ce.son_bakim_tarihi,
    sonraki_bakim_tarihi = ce.sonraki_bakim_tarihi, aciklama = ce.aciklama
FROM company_equipment ce
WHERE ce.id = pm.company_equipment_id;

ALTER TABLE project_machines
    ALTER COLUMN tip SET NOT NULL,
    ALTER COLUMN ad SET NOT NULL,
    ALTER COLUMN sahiplik SET NOT NULL,
    ALTER COLUMN sahiplik SET DEFAULT 'ozmal',
    ALTER COLUMN durum SET NOT NULL,
    ALTER COLUMN durum SET DEFAULT 'aktif',
    ADD CONSTRAINT project_machines_tip_check CHECK (tip IN ('arac','is_makinesi','ekipman')),
    ADD CONSTRAINT project_machines_sahiplik_check CHECK (sahiplik IN ('ozmal','kiralik')),
    ADD CONSTRAINT project_machines_durum_check CHECK (durum IN ('aktif','bakim','devre_disi'));

DROP INDEX IF EXISTS idx_pm_project_active;
DROP INDEX IF EXISTS idx_pm_equipment;
ALTER TABLE project_machines
    DROP COLUMN company_equipment_id,
    DROP COLUMN atanma_tarihi,
    DROP COLUMN ayrilma_tarihi,
    DROP COLUMN is_basi_tarihi,
    DROP COLUMN is_basi_detaylari;

CREATE INDEX idx_pm_project ON project_machines(project_id, tip);
