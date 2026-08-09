-- Migration 000030 — Şantiye Kasa Harcaması tablosu
--
-- manpower/equipment/work_entries ile aynı desen: günlük rapora bağlı satır,
-- Submitted rapor kilidine tabi (lock_submitted_daily_children). Proje-özel
-- seed verisi kasıtlı olarak yok — bu genel altyapı, tek bir demo projeye
-- göre değil, veri olsun olmasın her proje için doğru çalışmalı.

CREATE TABLE daily_cash_expenses (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    description     text NOT NULL,
    category        text NOT NULL DEFAULT 'Diğer',
    amount          numeric(12,2) NOT NULL CHECK (amount >= 0),
    receipt_no      text NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    row_version     integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_cash_expenses_report ON daily_cash_expenses (daily_report_id);

CREATE TRIGGER trg_daily_cash_expenses_updated_at BEFORE UPDATE ON daily_cash_expenses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_dce_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_cash_expenses
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();
