-- Faz 3.2 (E15) — toplantı aksiyonundan görev. Bir aksiyon satırı (toplantı + sıra
-- numarası) en fazla bir görevle eşleşir. Eşleme tablosu, görev tablosunu
-- değiştirmeden tekilleştirme sağlar.
CREATE TABLE meeting_action_tasks (
    meeting_id   uuid NOT NULL REFERENCES project_meetings (id) ON DELETE RESTRICT,
    action_index integer NOT NULL CHECK (action_index >= 0),
    task_id      uuid NOT NULL REFERENCES tasks (id) ON DELETE RESTRICT,
    created_by   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (meeting_id, action_index),
    CONSTRAINT uq_meeting_action_task UNIQUE (task_id)
);
