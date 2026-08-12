-- Migration 000035 — Saha Tutanakları (backend'e taşıma)
--
-- Önceden tamamen tarayıcı localStorage'ında tutulan bir özellikti; hiçbir
-- kullanıcı/cihaz arasında paylaşılmıyordu ve fotoğraflar base64 olarak
-- JSON içine gömülüydü. Bu migration, "Toplantı Tutanağı" (project_meetings)
-- ile aynı ruhta ama onay zinciri + hakedişe bağlanabilirlik gerektiren daha
-- zengin bir varlık olarak gerçek bir tabloya taşır. taseron_id artık gerçek
-- bir FK — eski localStorage sürümünde serbest metin/kırpılmış sahte bir
-- kimlikti.

CREATE TABLE saha_tutanaklari (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tip               text NOT NULL
                      CHECK (tip IN ('kaza_yangin_hirsizlik','ek_imalat','mesai','yevmiyeli')),
    baslik            text NOT NULL,
    tarih             date NOT NULL,
    taseron_id        uuid NULL REFERENCES subcontractors(id) ON DELETE SET NULL,
    kisim             text NULL,
    aciklama          text NOT NULL,
    tutar             numeric(18,2) NULL,
    birim             text NULL,
    miktar            numeric(18,3) NULL,
    -- Onay zinciri istemcide hesaplanıyordu (onayVer()); artık sunucuda
    -- (Submit/Decide) hesaplanır — tek doğruluk kaynağı burada.
    durum             text NOT NULL DEFAULT 'taslak'
                      CHECK (durum IN ('taslak','onay_sureci','onaylandi','reddedildi')),
    onay_zinciri      jsonb NOT NULL DEFAULT '[]'::jsonb,
    hakedise_eklendi  boolean NOT NULL DEFAULT false,
    created_by        uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz NULL,
    row_version       integer NOT NULL DEFAULT 1
);

CREATE INDEX idx_saha_tutanaklari_project
    ON saha_tutanaklari (project_id, tarih DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_saha_tutanaklari_taseron
    ON saha_tutanaklari (taseron_id) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_saha_tutanaklari_updated_at BEFORE UPDATE ON saha_tutanaklari
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Fotoğraflar mevcut polimorfik documents motoru üzerinden bağlanır
-- (entity_type='saha_tutanagi', entity_id=saha_tutanaklari.id) — ayrı bir
-- dosya/blob tablosu açılmaz. doc_category'ye bu amaçla bir değer eklenir.
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS','SahaTutanagi','Other'));
