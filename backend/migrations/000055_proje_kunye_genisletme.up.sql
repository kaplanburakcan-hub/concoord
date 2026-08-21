-- Migration 000055 — Proje künyesi bilgi alanlarını çeşitlendirme: proje
-- türü/kategorisi, toplam inşaat alanı ve kat/blok bilgisi. Görsel önizleme
-- kutuları (Proje Görseli, Konum/Vaziyet Planı Görseli) yeni sütun gerektirmez
-- — mevcut polimorfik documents motorunu kullanır (bkz. documents.docCategories,
-- "KonumGorseli" kategorisi ekleniyor; "ProjeGorseli" zaten Dashboard v2'den vardı).

ALTER TABLE projects
    ADD COLUMN proje_turu             text    NULL,
    ADD COLUMN toplam_insaat_alani_m2 numeric NULL,
    ADD COLUMN kat_blok_bilgisi       text    NULL;
