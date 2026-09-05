-- Migration 000061 — audit_logs.action CHECK kısıtına 'VIEW' eklendi.
-- Bugüne kadar audit yalnızca YAZMA (INSERT/UPDATE/DELETE) olaylarını
-- kaydediyordu. PDKS/GPS Puantaj (Blok 2, Aşama 2) hassas konum verisine
-- her OKUMA erişiminin de denetim izine düşmesini gerektiriyor
-- (bkz. internal/attendance/serialize.go) — bunun için ayrı, genel amaçlı
-- bir değer eklendi; başka modüller de aynı ihtiyaçla karşılaşırsa
-- audit.ActionView'ı yeniden kullanabilir.
ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_action_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_action_check
    CHECK (action IN ('INSERT', 'UPDATE', 'DELETE', 'VIEW'));
