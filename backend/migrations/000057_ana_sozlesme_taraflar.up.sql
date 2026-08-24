-- Ana Sözleşme'de taraf bilgileri: İşveren adı + Yüklenici Proje Sorumlusu.
-- Sözleşme görünümünde her zaman gösterilmesi/gerekmesi istendiği için
-- doğrudan project_main_contracts'a eklenir (proje profilindeki client_name
-- alanından bağımsız — sözleşme kendi içinde kendine yeterli olmalı).

ALTER TABLE project_main_contracts
    ADD COLUMN isveren_adi TEXT,
    ADD COLUMN yuklenici_proje_sorumlusu TEXT;
