DO $$
DECLARE v_proj uuid;
BEGIN
  SELECT id INTO v_proj FROM projects WHERE code='DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NOT NULL THEN
    DELETE FROM site_warehouse_movements WHERE project_id = v_proj;
    DELETE FROM site_warehouse_items WHERE project_id = v_proj;
  END IF;
END $$;

DROP TABLE IF EXISTS daily_cash_expenses;
