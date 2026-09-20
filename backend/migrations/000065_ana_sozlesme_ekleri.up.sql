-- Migration 000065 — Ana Sözleşme: ad-hoc tek-PDF alanı yerine gerçek
-- doküman/depolama motoru (documents) kullanılacak. pdf_dosya_url/adi hiç
-- gerçek bir upload'a bağlı değildi (frontend'de yalnızca dosya adı
-- tutuluyordu, gerçek bayt hiç yazılmıyordu) — bu yüzden kaldırılıyor.
ALTER TABLE project_main_contracts
    DROP COLUMN pdf_dosya_url,
    DROP COLUMN pdf_dosya_adi;

-- documents motoruna yeni kategori: "AnaSozlesmeEki" (entity_type='main_contract').
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','IdariHakedisBelgesi','ProjeGorseli','NakliyeIrsaliyesi',
        'KiralamaSozlesmesi','AnaSozlesmeEki','Other'));
