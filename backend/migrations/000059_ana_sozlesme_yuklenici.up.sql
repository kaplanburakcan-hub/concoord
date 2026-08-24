-- Ana Sözleşme Taraflar: "Yüklenici" (firma adı) "Yüklenici Proje Sorumlusu"
-- (kişi) alanından ayrı, bağımsız bir alandır.

ALTER TABLE project_main_contracts
    ADD COLUMN yuklenici_adi TEXT;
