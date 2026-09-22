ALTER TABLE projects
  DROP COLUMN IF EXISTS ulke,
  DROP COLUMN IF EXISTS il,
  DROP COLUMN IF EXISTS ilce,
  DROP COLUMN IF EXISTS enlem,
  DROP COLUMN IF EXISTS boylam;

ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','IdariHakedisBelgesi','ProjeGorseli','NakliyeIrsaliyesi',
        'KiralamaSozlesmesi','AnaSozlesmeEki','Other'));
