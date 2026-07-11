-- Faz 6 geri alma — saha raporlama tabloları ve kilit fonksiyonları.
DROP TABLE IF EXISTS weekly_reports;

DROP TRIGGER IF EXISTS trg_dw_lock ON daily_work_entries;
DROP TRIGGER IF EXISTS trg_de_lock ON daily_equipment;
DROP TRIGGER IF EXISTS trg_dm_lock ON daily_manpower;
DROP TRIGGER IF EXISTS trg_dr_lock ON daily_reports;
DROP FUNCTION IF EXISTS lock_submitted_daily_children();
DROP FUNCTION IF EXISTS lock_submitted_daily_report();

DROP TABLE IF EXISTS daily_work_entries;
DROP TABLE IF EXISTS daily_equipment;
DROP TABLE IF EXISTS daily_manpower;
DROP TABLE IF EXISTS daily_reports;
