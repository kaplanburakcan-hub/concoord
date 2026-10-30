-- Faz 3.4 (E16) — günlük raporda 'sorun' işareti. İşaretli rapor gönderilince
-- proje yöneticilerine bildirim gider; kayıt veya görev açılmaz.
ALTER TABLE daily_reports
    ADD COLUMN issue_flagged boolean NOT NULL DEFAULT false;
