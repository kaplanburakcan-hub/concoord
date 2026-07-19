-- ============================================================================
-- DEMO-01 — PV planı düzeltmesi
-- ============================================================================
-- Sorun: pv_plan_entries.planned_pct alanı O AYA AİT ARTIŞ yüzdesidir; sistem
-- bunları aydan aya toplayarak kümülatif PV eğrisini kurar (bkz. LinearPlanPct:
-- "ay başına eşit planlanan yüzde (Σ = 100)"). İlk demo verisinde kümülatif
-- değerler (4, 10, 18, 28…) girilmişti; toplanınca PV = BAC'ın %281'i oldu ve
-- SPI yapay olarak 0.213'e düştü.
--
-- Bu script yalnızca DEMO-01'in PV planını değiştirir; hakediş, İSG, görev ve
-- diğer hiçbir veriye dokunmaz.
--
-- Çalıştırma:
--   docker compose -f deploy/docker-compose.yml --env-file .env cp deploy/seed/fix-pv-plan.sql postgres:/tmp/fix.sql
--   docker compose -f deploy/docker-compose.yml --env-file .env exec postgres psql -U ipks -d ipks -f /tmp/fix.sql
-- ============================================================================

DO $$
DECLARE
  v_proj uuid;
BEGIN
  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-01' AND deleted_at IS NULL;
  IF v_proj IS NULL THEN
    RAISE EXCEPTION 'DEMO-01 projesi bulunamadı.';
  END IF;

  DELETE FROM pv_plan_entries WHERE project_id = v_proj;

  -- Aylık ARTIŞ yüzdeleri (toplam 100). 2026-07 sonu kümülatif %70 planlanmış olur.
  INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES
    (v_proj, DATE '2025-12-01',  4.000),
    (v_proj, DATE '2026-01-01',  6.000),
    (v_proj, DATE '2026-02-01',  8.000),
    (v_proj, DATE '2026-03-01', 10.000),
    (v_proj, DATE '2026-04-01', 11.000),
    (v_proj, DATE '2026-05-01', 11.000),
    (v_proj, DATE '2026-06-01', 10.000),
    (v_proj, DATE '2026-07-01', 10.000),
    (v_proj, DATE '2026-08-01',  9.000),
    (v_proj, DATE '2026-09-01',  8.000),
    (v_proj, DATE '2026-10-01',  7.000),
    (v_proj, DATE '2026-11-01',  6.000);

  RAISE NOTICE 'DEMO-01 PV planı düzeltildi (2026-07 kümülatif plan: %%70).';
END $$;
