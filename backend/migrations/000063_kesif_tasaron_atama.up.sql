-- Blok 2 Aşama 3 (ek) — Keşif kalemi × Taşeron/Sözleşme entegrasyonu.
--
-- Bir keşif kalemi (project_survey_items) bir taşerona atandığında, o an ki
-- poz_no/tanım/birim/miktar/birim_fiyat değerleriyle GERÇEK bir work_items
-- (hakediş poz) satırı oluşturulur ve buraya bağlanır. Keşif master BOQ
-- listesi olarak kalır; work_items ise ondan bağımsız olarak hakediş akışında
-- revize edilebilir (keşif tahmini ≠ sözleşme kalemi ayrımı bilinçli).
--
-- ON DELETE SET NULL: work_items satırı silinirse (nadiren, hakediş henüz
-- başlamamışsa RESTRICT ile korunuyor zaten) keşif kalemi yetim kalmaz,
-- yalnızca atanmamış duruma döner.
ALTER TABLE project_survey_items
    ADD COLUMN work_item_id UUID REFERENCES work_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_psi_work_item ON project_survey_items(work_item_id) WHERE work_item_id IS NOT NULL;
