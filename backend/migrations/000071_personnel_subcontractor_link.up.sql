-- Puantaj personelinin "Firma" alanı serbest metindi; hangi taşerona ait
-- olduğu bir bağ olarak tutulmuyordu (günlük raporda "Puantajdan Al" bu
-- yüzden tahmine dayanıyordu). subcontractor_id NULL = ana yüklenici.
ALTER TABLE project_personnel
    ADD COLUMN subcontractor_id uuid NULL REFERENCES subcontractors (id) ON DELETE SET NULL;

-- Mevcut kayıtlar: firma adı projedeki bir taşeronla birebir (büyük/küçük
-- harf ve boşluk farkı hariç) eşleşiyorsa otomatik bağlanır; kalanlar NULL
-- (ana yüklenici) kalır ve firma metni olduğu gibi korunur.
UPDATE project_personnel pp
SET subcontractor_id = s.id
FROM subcontractors s
WHERE s.project_id = pp.project_id
  AND s.deleted_at IS NULL
  AND pp.firma IS NOT NULL
  AND lower(btrim(pp.firma)) = lower(btrim(s.company_name));
