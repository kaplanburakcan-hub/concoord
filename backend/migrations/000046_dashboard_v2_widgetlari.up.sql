-- Migration 000046 — Kontrol paneli (Dashboard) v2: kaza kaydı, doküman
-- durumu, proje görseli. Kullanıcının paylaştığı örnek panel mockup'ından
-- ilham alınan, birebir değil KONSEPT olarak uyarlanan yeni widget'lar.
--
-- ohs_findings (İSG denetim bulguları) İLE ohs_accidents (fiili iş kazası
-- kaydı) KAVRAM OLARAK FARKLIDIR: findings saha denetiminde tespit edilen
-- ihlal/tehlikelerdir (kaza olmadan da açılır), accidents ise gerçekleşmiş
-- kaza olaylarının tarihidir — "kazasız gün" sayacı yalnızca bu tablodan
-- beslenir.

CREATE TABLE ohs_accidents (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    accident_date  date NOT NULL,
    description    text NOT NULL,
    created_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_ohs_accidents_project_date ON ohs_accidents (project_id, accident_date DESC)
    WHERE deleted_at IS NULL;
CREATE TRIGGER trg_ohs_accidents_updated_at BEFORE UPDATE ON ohs_accidents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Doküman durumu widget'ı: Taslak/Revizyon/Onaylı — resmi bir onay akışı
-- değil (documents genel amaçlı bir motor, MAR/hakediş gibi kendi onay
-- zincirleri ayrı tablolarda), yalnızca serbest bir durum etiketi.
ALTER TABLE documents ADD COLUMN approval_status text NOT NULL DEFAULT 'Taslak'
    CHECK (approval_status IN ('Taslak','Revizyon','Onaylı'));

-- Proje Görseli widget'ı: mevcut polimorfik documents motoru yeniden
-- kullanılır (entity_type='project', entity_id=projects.id) — yeni bir
-- dosya alanı/depolama yolu icat edilmez.
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','ProjeGorseli','Other'));
