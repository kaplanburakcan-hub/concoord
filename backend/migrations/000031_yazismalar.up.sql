-- Migration 000031 — Yazışmalar (Gelen/Giden Evrak)
--
-- Türkiye'deki kurumsal gelen-giden evrak defteri geleneği (EBYS) ile
-- inşaat sektöründeki RFI/Transmittal log pratiğini birleştirir: her kayıt
-- bir yöne (gelen/giden) aittir, projeye+yöne göre sıralı bir evrak no taşır,
-- ve "ball-in-court" mantığı için cevap_gerekli/cevap_tarihi/durum alanlarına
-- sahiptir. Ekler mevcut polimorfik documents motoru üzerinden bağlanır
-- (entity_type='correspondence', entity_id=correspondences.id) — ayrı bir
-- dosya tablosu açılmaz.

CREATE TABLE correspondences (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    direction      text NOT NULL CHECK (direction IN ('gelen','giden')),
    evrak_no       text NOT NULL,
    karsi_evrak_no text NULL,
    tarih          date NOT NULL,
    kayit_tarihi   date NOT NULL DEFAULT CURRENT_DATE,
    kurum_kisi     text NOT NULL,
    konu           text NOT NULL,
    kategori       text NOT NULL DEFAULT 'Genel'
                   CHECK (kategori IN ('Genel','Teknik','İdari','Mali','İSG','Onay Talebi')),
    durum          text NOT NULL DEFAULT 'Açık'
                   CHECK (durum IN ('Açık','Cevaplandı','Kapalı','Bilgi Amaçlı')),
    cevap_gerekli  boolean NOT NULL DEFAULT false,
    cevap_tarihi   date NULL,
    ilgili_yazi_id uuid NULL REFERENCES correspondences (id) ON DELETE SET NULL,
    dagitim        text NULL,
    notlar         text NULL,
    created_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);

-- Proje+yön içinde evrak no tekil (silinmiş kayıtlar numarayı serbest bırakmaz —
-- gerçek evrak defterlerinde numara asla yeniden kullanılmaz).
CREATE UNIQUE INDEX uq_correspondences_evrak_no
    ON correspondences (project_id, direction, evrak_no);

CREATE INDEX idx_correspondences_project
    ON correspondences (project_id, direction, tarih DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_correspondences_ilgili
    ON correspondences (ilgili_yazi_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_correspondences_durum
    ON correspondences (project_id, durum) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_correspondences_updated_at BEFORE UPDATE ON correspondences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
