ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','ProjeGorseli','NakliyeIrsaliyesi','KiralamaSozlesmesi','Other'));

ALTER TABLE idari_hakedisler
    DROP COLUMN kesintiler,
    DROP COLUMN onceki_hakedis_toplami,
    DROP COLUMN fiyat_farki_tutari,
    DROP COLUMN sozlesme_fiyatlari_tutari,
    DROP COLUMN hakedis_tarihi;
