-- Kullanıcı görevi (ünvan): Admin tarafından Kullanıcı Yönetimi'nden girilir.
ALTER TABLE users ADD COLUMN IF NOT EXISTS title text;
