-- schedule_items.weight numeric(12,4) idi — maksimum ~99.999.999. Keşiften
-- otomatik WBS oluşturma (miktar × birim_fiyat, gerçek sözleşme tutarları)
-- gerçek projelerde bunu kolayca aşıyor (örn. Betonarme kategorisi tek
-- başına 100M+ TL olabilir) ve Postgres "numeric field overflow" hatası
-- veriyordu. Repo genelindeki para kolonlarıyla (contracts.amount,
-- progress_payments.gross_cum vb.) aynı numeric(18,2) yerine 4 ondalık
-- hassasiyeti koruyoruz (weight yüzde/ağırlık olarak da kullanılabiliyor).
ALTER TABLE schedule_items ALTER COLUMN weight TYPE numeric(18,4);
