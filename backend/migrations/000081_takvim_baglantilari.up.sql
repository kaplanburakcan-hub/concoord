-- Faz 3.4 (E12) — kişiye özel takvim (ICS) bağlantıları.
-- Belirteç yalnızca özeti (SHA-256) saklanır; düz hâli bir kez gösterilir.
-- Bağlantı süresi dolar ya da iptal edilir; kullanıcının proje üyeliği biterse
-- bağlantı veri döndürmez.
CREATE TABLE calendar_feeds (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    token_hash   text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz NULL
);
CREATE INDEX idx_calendar_feeds_user ON calendar_feeds (user_id, project_id) WHERE revoked_at IS NULL;
