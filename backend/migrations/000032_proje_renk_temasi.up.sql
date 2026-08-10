-- Migration 000032 — Proje bazlı vurgu rengi/tema
--
-- Farklı projeler farklı vurgu renkleriyle ayırt edilebilsin diye
-- projects.accent_color (hex, ör. "#2f6fed") eklenir. NULL ise frontend
-- temanın varsayılan vurgu rengini kullanır.

ALTER TABLE projects ADD COLUMN accent_color text NULL;
