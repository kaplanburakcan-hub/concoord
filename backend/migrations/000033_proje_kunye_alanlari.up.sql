-- Migration 000033 — Proje künyesine sözleşme bedeli + aktif proje alanları
--
-- Sözleşme Bedeli, Yapım Bütçesi'nden (budget_total) ayrı bir alan olarak
-- eklenir. Statü "Active" olduğunda künyede zorunlu olarak doldurulması
-- istenen üç alan (yer teslim/iş başı tarihi, işveren proje sorumlusu,
-- şantiye şefi) eklenir; zorunluluk API katmanında uygulanır (bkz.
-- internal/projects/handler.go ValidateProject).

ALTER TABLE projects ADD COLUMN contract_amount   numeric(18,2) NULL;
ALTER TABLE projects ADD COLUMN site_handover_date date NULL;
ALTER TABLE projects ADD COLUMN client_rep_name    text NULL;
ALTER TABLE projects ADD COLUMN site_manager_name  text NULL;
