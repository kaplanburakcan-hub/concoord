-- Eski serbest firma metni "Ana yüklenici" (varsayılan placeholder / demo
-- verisi) artık subcontractor_id NULL + boş firma ile ifade edilir; aksi halde
-- arayüzde "taşeronla bağsız" uyarısı verirdi.
UPDATE project_personnel
SET firma = NULL
WHERE subcontractor_id IS NULL
  AND lower(btrim(firma)) IN ('ana yüklenici', 'ana yuklenici', 'anayüklenici', 'anayuklenici');
