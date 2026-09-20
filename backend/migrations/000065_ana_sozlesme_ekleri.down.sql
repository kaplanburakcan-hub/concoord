ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','IdariHakedisBelgesi','ProjeGorseli','NakliyeIrsaliyesi',
        'KiralamaSozlesmesi','Other'));

ALTER TABLE project_main_contracts
    ADD COLUMN pdf_dosya_url text,
    ADD COLUMN pdf_dosya_adi text;
