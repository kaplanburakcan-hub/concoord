-- Sigorta ve Poliçeler (proje bazlı sigorta poliçesi takibi) +
-- Ana Sözleşme'ye "kaydet ve kilitle" bayrağı.

CREATE TABLE IF NOT EXISTS insurance_policies (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    police_turu          TEXT        NOT NULL CHECK (police_turu IN (
                              'car_ear', 'isveren_mali_sorumluluk',
                              'ucuncu_sahis_mali_sorumluluk', 'nakliyat', 'diger')),
    sigorta_sirketi      TEXT        NOT NULL,
    police_no            TEXT        NOT NULL,

    baslangic_tarihi     DATE,
    bitis_tarihi         DATE,

    teminat_bedeli       NUMERIC(20,2),
    teminat_para_birimi  TEXT        NOT NULL DEFAULT 'TRY',
    prim_tutari          NUMERIC(20,2),
    prim_para_birimi     TEXT        NOT NULL DEFAULT 'TRY',

    durum                TEXT        NOT NULL DEFAULT 'aktif'
                                      CHECK (durum IN ('aktif','suresi_doldu','iptal')),
    aciklama             TEXT,

    pdf_dosya_url        TEXT,
    pdf_dosya_adi        TEXT,

    created_by           UUID        REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ,
    row_version          INTEGER     NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_insurance_policies_project
    ON insurance_policies(project_id) WHERE deleted_at IS NULL;

-- Ana Sözleşme: kaydedilince otomatik kilitlenir (bkz. contracts.Upsert).
-- created_by/updated_by kolonları migration 000018'de zaten vardı ama hiç
-- kullanılmıyordu — bu değişiklikle birlikte dolduruluyor (kilitleyen/son
-- revize eden kullanıcıyı göstermek için).
ALTER TABLE project_main_contracts
    ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT FALSE;
