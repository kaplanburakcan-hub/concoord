-- ============================================================================
-- 000027 — Günlük rapor kapak fotoğrafı
-- ============================================================================
-- Günlük saha raporuna opsiyonel bir kapak fotoğrafı eklenir.
-- Fotoğraf files tablosuna kaydedilir; kural dışı boyut/format uygulama
-- katmanında reddedilir (NOT NULL kullanılmaz, eski kayıtlar etkilenmez).
-- ============================================================================

ALTER TABLE daily_reports
    ADD COLUMN IF NOT EXISTS cover_photo_file_id uuid NULL
        REFERENCES files (id) ON DELETE SET NULL;

COMMENT ON COLUMN daily_reports.cover_photo_file_id IS
    'Günlük rapor kapak fotoğrafı — opsiyonel, files tablosuna referans';
