-- Zimmet iadesi: teslim edilen zimmet kalemi iade edildiğinde iade tarihi ve
-- notu kaydedilir. iade_tarihi NULL olan onaylı zimmetler "açık zimmet" sayılır
-- (Malzeme - Envanter raporları, Depo-Stok bölümü).

ALTER TABLE saha_tutanaklari
    ADD COLUMN iade_tarihi date NULL,
    ADD COLUMN iade_notu   text NULL;
