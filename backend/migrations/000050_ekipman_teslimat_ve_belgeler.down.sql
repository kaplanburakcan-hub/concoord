ALTER TABLE project_machines
    DROP COLUMN from_transfer_id,
    DROP COLUMN teslim_alindi_tarihi;

ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','ProjeGorseli','Other'));
