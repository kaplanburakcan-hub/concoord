-- Migration 000048 — Makine/Ekipman/Araç Envanteri Faz A: mevcut
-- project_machines kayıtlarını company_equipment'e taşı, project_machines'i
-- kimlik/mülkiyet bilgisi taşıyan bir tablodan "proje ataması" kaydına
-- indirger.

-- Adım 1: mevcut her project_machines satırından BİR company_equipment
-- satırı üret. Aynı id'yi yeniden kullanıyoruz (project_machines.id ==
-- company_equipment.id) — böylece ayrıca bir eşleştirme/join adımına
-- gerek kalmadan project_machines.company_equipment_id'yi doğrudan
-- project_machines.id'den backfill edebiliriz.
INSERT INTO company_equipment (
    id, tip, ad, plaka, seri_no, marka, model, uretim_yili, sahiplik,
    tedarikci, gunluk_ucret, durum, son_bakim_tarihi, sonraki_bakim_tarihi,
    aciklama, current_project_id, created_at, updated_at
)
SELECT
    id, tip, ad, plaka, seri_no, marka, model, uretim_yili, sahiplik,
    tedarikci, gunluk_ucret, durum, son_bakim_tarihi, sonraki_bakim_tarihi,
    aciklama, project_id, created_at, updated_at
FROM project_machines;

-- Adım 2: project_machines'e atama sütunlarını ekle.
ALTER TABLE project_machines
    ADD COLUMN company_equipment_id uuid REFERENCES company_equipment(id),
    ADD COLUMN atanma_tarihi date NOT NULL DEFAULT CURRENT_DATE,
    ADD COLUMN ayrilma_tarihi date,
    ADD COLUMN is_basi_tarihi date,
    ADD COLUMN is_basi_detaylari jsonb;

-- Adım 3: backfill — id'ler adım 1'de eşleştirildiği için doğrudan kopya.
UPDATE project_machines SET company_equipment_id = id;
ALTER TABLE project_machines ALTER COLUMN company_equipment_id SET NOT NULL;

-- atanma_tarihi'ni gerçek oluşturulma tarihine göre düzelt (yukarıdaki
-- DEFAULT CURRENT_DATE sadece yeni satırlar için; mevcut kayıtlar
-- gerçek geçmişini yansıtsın).
UPDATE project_machines SET atanma_tarihi = created_at::date;

-- Adım 4: artık company_equipment'te yaşayan kimlik/mülkiyet/durum
-- sütunlarını kaldır.
DROP INDEX IF EXISTS idx_pm_project;
ALTER TABLE project_machines
    DROP COLUMN tip,
    DROP COLUMN ad,
    DROP COLUMN plaka,
    DROP COLUMN marka,
    DROP COLUMN model,
    DROP COLUMN seri_no,
    DROP COLUMN uretim_yili,
    DROP COLUMN sahiplik,
    DROP COLUMN tedarikci,
    DROP COLUMN gunluk_ucret,
    DROP COLUMN durum,
    DROP COLUMN son_bakim_tarihi,
    DROP COLUMN sonraki_bakim_tarihi,
    DROP COLUMN aciklama;

CREATE INDEX idx_pm_project_active ON project_machines (project_id) WHERE ayrilma_tarihi IS NULL;
CREATE INDEX idx_pm_equipment ON project_machines (company_equipment_id);
