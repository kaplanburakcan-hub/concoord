-- Migration 000045 — Nakit Akış Faz E: Sabit Giderler (araç kiraları,
-- endirekt personel, mobilizasyon sarf vb. tekrarlayan aylık giderler).
--
-- Bu tablo cash_events'e YAZILMAZ: Render'da arka plan işçisi (cmd/worker)
-- çalışmadığı için, aylık tekrarlayan giderleri önceden satır satır üretecek
-- bir cron kurulamaz. Bunun yerine nakit akış raporu (Faz F) istenen tarih
-- aralığı için bu kayıtları anlık/sanal olarak ay ay genişletir — hiçbir
-- zamanlanmış iş gerektirmez.

CREATE TABLE fixed_expenses (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    label                 text NOT NULL,
    amount                numeric(18,2) NOT NULL,
    category              text NOT NULL DEFAULT 'Diğer',
    expense_day_of_month  integer NOT NULL CHECK (expense_day_of_month BETWEEN 1 AND 28),
    start_date            date NOT NULL,
    end_date              date NULL,
    active                boolean NOT NULL DEFAULT true,
    created_by            uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz NULL,
    row_version           integer NOT NULL DEFAULT 1,
    CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE INDEX idx_fixed_expenses_project ON fixed_expenses (project_id) WHERE deleted_at IS NULL;
