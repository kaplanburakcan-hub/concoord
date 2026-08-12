-- Migration 000044 — Nakit Akış Faz D: İdari Hakedişler (işverenden gelen
-- tahsilat — nakit akışının tek "in" kaynağı). Kapsam bilinçli olarak dar:
-- idarenin kendi onay süreci sistem dışında gerçekleşiyor (idare zaten
-- onaylamış), bu yüzden burada ayrı bir çok adımlı onay zinciri kurulmuyor;
-- doğrudan "onaylanmış kayıt girişi" formu. Fatura mevcut polimorfik
-- documents motoruyla bağlanır (entity_type='idari_hakedis_fatura').

CREATE TABLE idari_hakedisler (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    donem_no           integer NOT NULL,
    aciklama           text NOT NULL,
    tutar              numeric(18,2) NOT NULL, -- KDV dahil, fiilen tahsil edilen/edilecek toplam
    kdv_pct            numeric(5,2) NOT NULL DEFAULT 20,
    fatura_no          text NULL,
    gelen_odeme_tarihi date NULL,
    created_by         uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz NULL,
    row_version        integer NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX uq_idari_hakedis_donem ON idari_hakedisler (project_id, donem_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_idari_hakedis_project ON idari_hakedisler (project_id) WHERE deleted_at IS NULL;

ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','Other'));
