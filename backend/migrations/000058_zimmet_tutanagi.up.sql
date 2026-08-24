-- Zimmet Tutanağı — saha_tutanaklari'ne yeni bir tip. Depo Raporları ve
-- Saha Tutanakları sayfaları arasında paylaşılan aynı kayıt: personel
-- seçilince firması (project_personnel.firma) otomatik gösterilir, kayıt
-- oluşunca ilgili firmanın (subcontractors.company_name eşleşmesiyle)
-- tanımlı kullanıcılarına bildirim gider (bkz. internal/tutanaklar/handler.go).

ALTER TABLE saha_tutanaklari DROP CONSTRAINT saha_tutanaklari_tip_check;
ALTER TABLE saha_tutanaklari ADD CONSTRAINT saha_tutanaklari_tip_check
    CHECK (tip IN ('kaza_yangin_hirsizlik','ek_imalat','mesai','yevmiyeli','zimmet'));

ALTER TABLE saha_tutanaklari
    ADD COLUMN personel_id uuid NULL REFERENCES project_personnel(id) ON DELETE SET NULL;

CREATE INDEX idx_saha_tutanaklari_personel
    ON saha_tutanaklari (personel_id) WHERE deleted_at IS NULL;
