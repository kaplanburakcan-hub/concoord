-- Faz 4 / Migration 5 — Görev Yönetimi + Bildirim Motoru
-- Plan §6.5, §8 (Faz 4). Bildirim motoru merkezi altyapıdır: sonraki tüm
-- modüller (MAR, İSG, hakediş...) notify servisini kullanır. Kuyruk PostgreSQL
-- üzerindedir (Redis bağımlılığı yok, Plan §2); worker FOR UPDATE SKIP LOCKED
-- ile işleri çeker.

-- ---------------------------------------------------------------------------
-- Görevler (Kanban)
-- ---------------------------------------------------------------------------
CREATE TABLE tasks (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    title                text NOT NULL,
    description          text NULL,
    status               text NOT NULL DEFAULT 'Backlog'
                         CHECK (status IN ('Backlog','Todo','InProgress','Review','Done')),
    priority             text NOT NULL DEFAULT 'Normal'
                         CHECK (priority IN ('Low','Normal','High','Urgent')),
    due_date             date NULL,
    created_by           uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    kanban_order         double precision NOT NULL DEFAULT 0, -- kolon içi sıra (araya ekleme: ortalama)
    deadline_notified_at timestamptz NULL,                    -- hatırlatma tekrarını engeller
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz NULL,
    row_version          integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_tasks_project_status ON tasks (project_id, status, kanban_order) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_due ON tasks (due_date) WHERE deleted_at IS NULL AND status <> 'Done';
CREATE TRIGGER trg_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE task_assignees (
    task_id    uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE, -- bağ tablosu; görev soft-delete olduğundan kaskad güvenli
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, user_id)
);
CREATE INDEX idx_task_assignees_user ON task_assignees (user_id);

CREATE TABLE task_comments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     uuid NOT NULL REFERENCES tasks (id) ON DELETE RESTRICT,
    author_id   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    body        text NOT NULL,
    mentions    jsonb NOT NULL DEFAULT '[]'::jsonb, -- çözümlenen kullanıcı id listesi
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL,
    row_version integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_task_comments_task ON task_comments (task_id, created_at) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_task_comments_updated_at BEFORE UPDATE ON task_comments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Bildirimler (in-app kaydı; e-posta/SMS gönderimi kuyruk üzerinden asenkron)
-- ---------------------------------------------------------------------------
CREATE TABLE notifications (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    type        text NOT NULL,      -- task_assigned | task_mention | task_comment | task_deadline | ...
    title       text NOT NULL,
    body        text NULL,
    entity_type text NULL,          -- 'tasks', 'progress_payments', ...
    entity_id   uuid NULL,
    project_id  uuid NULL REFERENCES projects (id) ON DELETE RESTRICT,
    channel     text NOT NULL DEFAULT 'InApp'
                CHECK (channel IN ('InApp','Email','SMS')),
    read_at     timestamptz NULL,
    sent_at     timestamptz NULL,   -- Email/SMS için worker doldurur; InApp = created_at
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz NULL,
    row_version integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_notifications_user_unread ON notifications (user_id, created_at DESC)
    WHERE deleted_at IS NULL AND channel = 'InApp';
CREATE TRIGGER trg_notifications_updated_at BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Kanal tercihleri: satır yoksa varsayılan geçerlidir (InApp+Email açık, SMS kapalı).
CREATE TABLE notification_preferences (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel    text NOT NULL CHECK (channel IN ('InApp','Email','SMS')),
    enabled    boolean NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, channel)
);

-- ---------------------------------------------------------------------------
-- İş kuyruğu (PostgreSQL tabanlı; Plan §2 "Go worker + PostgreSQL kuyruk").
-- Worker: FOR UPDATE SKIP LOCKED ile atomik iş alma; hata = üstel geri çekilme.
-- ---------------------------------------------------------------------------
CREATE TABLE job_queue (
    id         bigserial PRIMARY KEY,
    kind       text NOT NULL,                 -- send_email | send_sms | ...
    payload    jsonb NOT NULL DEFAULT '{}'::jsonb,
    status     text NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','running','done','failed')),
    run_at     timestamptz NOT NULL DEFAULT now(),
    attempts   integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 5,
    last_error text NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_job_queue_ready ON job_queue (run_at) WHERE status = 'pending';
