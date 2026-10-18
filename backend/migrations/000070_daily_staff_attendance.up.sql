-- Günlük rapor: Yüklenici ve Müşavir kadro personelinin (Proje Paydaşları'nda
-- tanımlı kişiler) o günkü durumu (Mevcut/Devamsız/İzinli/Raporlu). Puantajdaki
-- durum kutularıyla aynı dört değer. İsim/unvan raporun gönderildiği andaki
-- haliyle saklanır (paydaş sonradan silinse/değişse de rapor bozulmaz).
CREATE TABLE daily_staff_attendance (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    stakeholder_id  uuid NULL,
    grup            text NOT NULL CHECK (grup IN ('yuklenici', 'musavir')),
    ad_soyad        text NOT NULL,
    unvan           text NULL,
    durum           text NOT NULL DEFAULT 'mevcut'
                    CHECK (durum IN ('mevcut', 'devamsiz', 'izinli', 'raporlu')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    row_version     integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_staff_report ON daily_staff_attendance (daily_report_id);
CREATE TRIGGER trg_daily_staff_updated_at BEFORE UPDATE ON daily_staff_attendance
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_dstaff_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_staff_attendance
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();
