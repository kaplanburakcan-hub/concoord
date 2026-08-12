-- Migration 000039 — Proje Fotoğrafları
--
-- "Fotoğraflar" sayfası önceden ComingSoon placeholder'ıydı. Yeni bir
-- tablo gerekmiyor: mevcut polimorfik documents motoru
-- (entity_type='project_photo', entity_id=<project_id>) kullanılıyor,
-- sadece Saha/İmalat/Denetim ayrımı için üç yeni doc_category eklenir.
--
-- Ayrıca bu ALTER, migration 000035'te DB CHECK constraint'ine eklenmiş
-- ama backend/internal/documents/handler.go'daki docCategories Go
-- map'ine hiç eklenmemiş olan 'SahaTutanagi' değerinin CHECK'te zaten
-- var olduğunu korur (Go tarafı ayrı bir commit'te düzeltiliyor).
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing',
        'Delivery','OHS','SahaTutanagi','SahaFotografi','ImalatFotografi',
        'DenetimFotografi','Other'));
