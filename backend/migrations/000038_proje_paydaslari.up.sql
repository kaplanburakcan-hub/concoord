-- Migration 000038 — Proje Paydaşları (backend'e taşıma)
--
-- ProjePaydaslariPage.tsx'in 9 kategorisinden 8'i (İşveren, Müşavir, Yapı
-- Denetim, Müellif, Danışmanlar, İSG-OSGB, Tedarikçiler, taşeron
-- alt-personeli) tamamen tarayıcı localStorage'ında tutuluyordu. Tüm
-- kategoriler tek bir ortak şekli (Paydas) paylaştığından tek tablo yeterli.
--
-- alt_kirilim_id bilinçli olarak FK DEĞİL: ya statik bir alt kırılım id'si
-- (ör. "yk_pm") ya da (kategori="alt_yuklenici" + tip="kisi" satırlarında)
-- gerçek subcontractors.id'nin metin hali olabilir — önceki localStorage
-- sürümündeki örtük çift-kullanım korunuyor.

CREATE TABLE project_stakeholders (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id     uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    kategori_id    text NOT NULL,
    alt_kirilim_id text NULL,
    tip            text NOT NULL CHECK (tip IN ('kisi', 'firma')),
    ad             text NOT NULL,
    soyad          text NULL,
    unvan          text NULL,
    firma_adi      text NULL,
    telefon        text NULL,
    email          text NULL,
    notlar         text NULL,
    created_by     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz NULL,
    row_version    integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_project_stakeholders_project
    ON project_stakeholders (project_id, kategori_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_project_stakeholders_updated_at BEFORE UPDATE ON project_stakeholders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
