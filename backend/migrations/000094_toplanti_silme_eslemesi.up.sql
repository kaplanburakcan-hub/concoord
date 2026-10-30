-- Toplantılar sert silinir (soft delete yok). Toplantı silinince yalnız aksiyon-görev
-- eşlemesi kaldırılır; oluşturulmuş görevler kalır.
ALTER TABLE meeting_action_tasks
    DROP CONSTRAINT meeting_action_tasks_meeting_id_fkey,
    ADD CONSTRAINT meeting_action_tasks_meeting_id_fkey
        FOREIGN KEY (meeting_id) REFERENCES project_meetings (id) ON DELETE CASCADE;
