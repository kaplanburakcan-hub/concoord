-- Çeyrek yıl ve özel dönem raporları: aylık rapor tablosu zaten period_start /
-- period_end tutuyor; yalnızca raporun türü eklenir. year/month sütunları
-- dönemin başlangıç tarihinden doldurulmaya devam eder (liste sıralaması için).
ALTER TABLE monthly_reports
    ADD COLUMN kind text NOT NULL DEFAULT 'Monthly'
    CHECK (kind IN ('Monthly','Quarterly','Custom'));
