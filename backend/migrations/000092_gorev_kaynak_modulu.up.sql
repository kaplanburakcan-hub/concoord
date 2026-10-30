-- Görev kaynak modülü (B21). Mevcut görevler 'elle' olarak kalır; eşleme tablosu olan
-- görevler (toplantı aksiyonu, İSG bulgusu) geriye dönük işaretlenir.
ALTER TABLE tasks
    ADD COLUMN source_module text NOT NULL DEFAULT 'elle'
        CHECK (source_module IN ('elle', 'toplanti', 'isg'));
UPDATE tasks SET source_module = 'toplanti'
    WHERE id IN (SELECT task_id FROM meeting_action_tasks);
UPDATE tasks SET source_module = 'isg'
    WHERE id IN (SELECT task_id FROM ohs_finding_tasks);
