-- Migration 000050 — Makine/Ekipman/Araç Envanteri Faz C: teslim alma
-- bildirimi + nakliye irsaliyesi + kiralama sözleşmesi belgeleri.
--
-- documents motoruna iki yeni kategori: "NakliyeIrsaliyesi"
-- (entity_type='equipment_transfer_requests') ve "KiralamaSozlesmesi"
-- (entity_type='company_equipment').
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','ProjeGorseli','NakliyeIrsaliyesi','KiralamaSozlesmesi','Other'));

-- Bir atamanın onaylı bir transferden mi geldiğini izlemek için (teslim
-- alındı butonunun görünürlüğü buna bağlı) + teslim onay tarihi.
ALTER TABLE project_machines
    ADD COLUMN from_transfer_id uuid NULL REFERENCES equipment_transfer_requests(id) ON DELETE SET NULL,
    ADD COLUMN teslim_alindi_tarihi date NULL;
