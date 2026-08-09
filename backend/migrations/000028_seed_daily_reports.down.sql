-- 000028 geri alma: DEMO-04 için eklenen günlük rapor seed verisini sil.
DO $$
DECLARE v_proj uuid;
BEGIN
  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NULL THEN RETURN; END IF;
  DELETE FROM daily_reports
  WHERE project_id = v_proj
    AND report_date BETWEEN '2026-07-27' AND '2026-08-09'
    AND revision_no = 1
    AND status = 'Draft';
END $$;
