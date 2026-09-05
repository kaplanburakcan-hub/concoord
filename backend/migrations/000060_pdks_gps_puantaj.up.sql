-- Migration 000060 — PDKS/GPS Puantaj (Blok 2, Aşama 2). Konum SADECE
-- giriş/çıkış anında, tek atış olarak alınır (watchPosition/arka plan
-- konumu YOK — bkz. backend/internal/attendance). Bu, mevcut manuel
-- haftalık puantaj sistemiyle (project_puantaj, migration 000021) BİLİNÇLİ
-- olarak AYRI bir sistemdir — ikisi de project_personnel'i paylaşır ama
-- birbirinin yerini almaz.

-- Şantiye sınırı (geofence) — proje başına birden fazla olabilir (ör. şantiye
-- girişi + şantiye dışı depo sahası).
CREATE TABLE site_geofences (
    id         uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid             NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       text             NOT NULL,
    center_lat double precision NOT NULL,
    center_lng double precision NOT NULL,
    radius_m   integer          NOT NULL DEFAULT 300,
    is_active  boolean          NOT NULL DEFAULT true,
    created_at timestamptz      NOT NULL DEFAULT now(),
    updated_at timestamptz      NOT NULL DEFAULT now()
);
CREATE INDEX idx_geofences_project ON site_geofences (project_id);

-- Ham giriş/çıkış olayları. client_uuid, offline kuyruktan tekrar gönderilen
-- kaydın idempotent kalmasını garanti eder (UNIQUE) — aynı kayıt 3 kez
-- gelirse tek satır oluşur, sunucu ikinci ve üçüncü isteği "zaten var" olarak
-- (200, hata değil) yanıtlar.
CREATE TABLE attendance_events (
    id          uuid             PRIMARY KEY DEFAULT gen_random_uuid(),
    client_uuid uuid             NOT NULL,
    project_id  uuid             NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    geofence_id uuid             REFERENCES site_geofences(id) ON DELETE SET NULL,
    person_id   uuid             NOT NULL REFERENCES project_personnel(id) ON DELETE CASCADE,
    event_type  text             NOT NULL CHECK (event_type IN ('in','out')),
    source      text             NOT NULL CHECK (source IN ('qr','manual','import')),

    -- Konum yalnızca bu tek olay anına ait — sürekli iz sürme YOK. 730 günü
    -- geçen kayıtlarda bu dört alan bir arka plan işiyle NULL'a çekilir
    -- (bkz. attendance_retention_settings), attendance_days etkilenmez.
    lat         double precision,
    lng         double precision,
    accuracy_m  real,
    distance_m  real,
    geofence_ok boolean,

    captured_at timestamptz      NOT NULL,
    received_at timestamptz      NOT NULL DEFAULT now(),
    device_id   text,
    recorded_by uuid REFERENCES users(id),
    note        text,
    UNIQUE (client_uuid)
);
CREATE INDEX idx_attendance_events_lookup ON attendance_events (project_id, person_id, captured_at);
CREATE INDEX idx_attendance_events_geofence ON attendance_events (geofence_id);

-- Günlük türetilmiş puantaj — ham olaylardan hesaplanır, elle düzeltme ham
-- kaydı SİLMEZ/DEĞİŞTİRMEZ, yalnızca adjusted_hours'a yansır.
CREATE TABLE attendance_days (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid          NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    person_id       uuid          NOT NULL REFERENCES project_personnel(id) ON DELETE CASCADE,
    work_date       date          NOT NULL,
    derived_hours   numeric(5,2),
    adjusted_hours  numeric(5,2),
    overtime_hours  numeric(5,2)  NOT NULL DEFAULT 0,
    status          text          NOT NULL DEFAULT 'derived' CHECK (status IN ('derived','adjusted','approved')),
    adjusted_by     uuid REFERENCES users(id),
    adjusted_reason text,
    approved_by     uuid REFERENCES users(id),
    approved_at     timestamptz,
    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    UNIQUE (project_id, person_id, work_date)
);
CREATE INDEX idx_attendance_days_project_date ON attendance_days (project_id, work_date);

-- Şantiye panosunda dönen tek kullanımlık, 60 saniyelik QR — statik QR
-- üretilmez, her istek yeni bir token döner.
CREATE TABLE attendance_qr_tokens (
    token       text        PRIMARY KEY,
    geofence_id uuid        NOT NULL REFERENCES site_geofences(id) ON DELETE CASCADE,
    issued_at   timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL
);
CREATE INDEX idx_qr_tokens_geofence ON attendance_qr_tokens (geofence_id, expires_at);

-- KVKK saklama süresi ayarı — tekil satır (singleton), varsayılan 730 gün.
-- Arka plan işi bu değeri okuyup süresi dolan attendance_events'te konum
-- alanlarını NULL'a çeker (bkz. Aşama 2.4, Adım 3'te uygulanacak).
CREATE TABLE attendance_retention_settings (
    id             boolean     PRIMARY KEY DEFAULT true CHECK (id),
    retention_days integer     NOT NULL DEFAULT 730 CHECK (retention_days > 0),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    updated_by     uuid REFERENCES users(id)
);
INSERT INTO attendance_retention_settings (id) VALUES (true);
