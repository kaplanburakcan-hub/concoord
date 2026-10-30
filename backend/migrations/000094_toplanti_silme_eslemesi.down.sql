-- geri alma: eşleme yabancı anahtarını RESTRICT'e döndür
ALTER TABLE meeting_action_tasks
    DROP CONSTRAINT meeting_action_tasks_meeting_id_fkey,
    ADD CONSTRAINT meeting_action_tasks_meeting_id_fkey
        FOREIGN KEY (meeting_id) REFERENCES project_meetings (id) ON DELETE RESTRICT;
