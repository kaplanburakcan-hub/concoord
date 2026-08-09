-- Migration 000028 — Demo seed: günlük saha raporları (Hafta 31–32 / 2026)
-- WHERE NOT EXISTS kullanılır; partial unique index ile uyumlu, idempotent.

DO $$
DECLARE
  v_proj  uuid;
  v_admin uuid;
  dr_id   uuid;
BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  SELECT id INTO v_proj  FROM projects WHERE code = 'DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NULL OR v_admin IS NULL THEN RETURN; END IF;

  -- ── Günlük raporları ekle (Draft, WHERE NOT EXISTS → idempotent) ──────────

  INSERT INTO daily_reports
    (id, project_id, report_date, revision_no, status, author_id,
     weather, temperature_min, temperature_max, notes)
  SELECT gen_random_uuid(), v_proj, d.rdate::date, 1, 'Draft', v_admin,
         d.wx::jsonb, d.tmin, d.tmax, d.notes
  FROM (VALUES
    ('2026-07-27','{"condition":"Parçalı Bulutlu"}',24,33,
     'Bodrum kat kalıp sökme tamamlandı. Beton dökümüne hazırlık yapıldı.'),
    ('2026-07-28','{"condition":"Açık ve Güneşli"}',26,36,
     'B1 aksı kolonları beton dökümü yapıldı. 4 adet kolon tamamlandı.'),
    ('2026-07-29','{"condition":"Açık ve Güneşli"}',27,37,
     'B2–B3 aksı kolon kalıpları kuruldu. Demir imalatı devam ediyor.'),
    ('2026-07-30','{"condition":"Parçalı Bulutlu"}',25,34,
     'Zemin kat döşeme betonarme imalatı: 3 aks tamamlandı.'),
    ('2026-07-31','{"condition":"Açık ve Güneşli"}',26,35,
     '1. kat perde beton kalıp montajı. Elektrik tesisatı kanalları açıldı.'),
    ('2026-08-01','{"condition":"Yağmurlu"}',19,26,
     'Hava koşulları nedeniyle beton dökümü yapılamadı. İç mekan imalatlarına devam edildi.'),
    ('2026-08-02','{"condition":"Parçalı Bulutlu"}',22,30,
     'A aksı 1. kat kolon beton dökümü tamamlandı. Haftalık kontrol yapıldı.'),
    ('2026-08-03','{"condition":"Açık ve Güneşli"}',25,35,
     '1. kat kirişler ve döşeme kalıpları kuruldu. 42 işçi sahada.'),
    ('2026-08-04','{"condition":"Açık ve Güneşli"}',26,36,
     'C aksı döşeme demiri bağlandı. Alt kat perde duvar sıvası başlandı.'),
    ('2026-08-05','{"condition":"Parçalı Bulutlu"}',24,33,
     '1. kat plak döşeme beton dökümü: 180 m². Vibrasyon işlemi tamamlandı.'),
    ('2026-08-06','{"condition":"Açık ve Güneşli"}',27,38,
     '2. kat kalıp çalışması başlandı. Elektrik ana dağıtım panosu montajı yapıldı.'),
    ('2026-08-07','{"condition":"Açık ve Güneşli"}',28,39,
     'Mekanik tesisat (yangın söndürme) boru hatları B bloğa çekildi.'),
    ('2026-08-09','{"condition":"Açık ve Güneşli"}',26,35,
     'Hafta kapanış kontrol ve puantaj. Güvenlik toplantısı yapıldı.')
  ) AS d(rdate, wx, tmin, tmax, notes)
  WHERE NOT EXISTS (
    SELECT 1 FROM daily_reports x
    WHERE x.project_id = v_proj
      AND x.report_date = d.rdate::date
      AND x.revision_no = 1
      AND x.deleted_at IS NULL
  );

  -- ── Alt satırları ekle (Draft raporlar için, daha önce eklenmemişse) ──────

  FOR dr_id IN
    SELECT dr.id FROM daily_reports dr
    WHERE dr.project_id = v_proj
      AND dr.report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND dr.revision_no = 1
      AND dr.deleted_at IS NULL
      AND dr.status != 'Submitted'
      AND NOT EXISTS (SELECT 1 FROM daily_manpower dm WHERE dm.daily_report_id = dr.id)
  LOOP
    INSERT INTO daily_manpower (daily_report_id, trade, headcount)
    VALUES
      (dr_id, 'Betonarme',    18),
      (dr_id, 'Kalıpçı',      10),
      (dr_id, 'Demirci',       8),
      (dr_id, 'Elektrikçi',    5),
      (dr_id, 'Tesisatçı',     4),
      (dr_id, 'İşçi (Genel)', 6);
  END LOOP;

  FOR dr_id IN
    SELECT dr.id FROM daily_reports dr
    WHERE dr.project_id = v_proj
      AND dr.report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND dr.revision_no = 1
      AND dr.deleted_at IS NULL
      AND dr.status != 'Submitted'
      AND NOT EXISTS (SELECT 1 FROM daily_equipment de WHERE de.daily_report_id = dr.id)
  LOOP
    INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours)
    VALUES
      (dr_id, 'Beton Pompası', 1, 6.0),
      (dr_id, 'Transmikser',   3, 8.0),
      (dr_id, 'Tower Crane',   1, 9.0),
      (dr_id, 'Kompresör',     1, 4.5),
      (dr_id, 'Forklift',      1, 5.0);
  END LOOP;

  FOR dr_id IN
    SELECT dr.id FROM daily_reports dr
    WHERE dr.project_id = v_proj
      AND dr.report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND dr.revision_no = 1
      AND dr.deleted_at IS NULL
      AND dr.status != 'Submitted'
      AND NOT EXISTS (SELECT 1 FROM daily_work_entries dwe WHERE dwe.daily_report_id = dr.id)
  LOOP
    INSERT INTO daily_work_entries (daily_report_id, description, location, qty, unit)
    VALUES
      (dr_id, 'Kolon Beton Dökümü (C25/30)',         'B Blok – 1. Kat',   12.5, 'm³'),
      (dr_id, 'Döşeme Kalıp Montajı',                 'A Blok – 1. Kat',  180.0, 'm²'),
      (dr_id, 'Betonarme Demir İmalatı (B420C)',      'B Blok',           1850.0, 'kg'),
      (dr_id, 'Perde Duvar Sıva (1. Kat)',            'A Blok İç Mekan',   45.0, 'm²'),
      (dr_id, 'Yangın Söndürme Boru Tesisatı (DN50)', 'B Blok Bodrum',     28.0, 'm');
  END LOOP;

  -- ── Mevcut boş haftalık rapor snapshot'larını yeniden oluştur ────────────

  UPDATE weekly_reports wr
  SET snapshot = (
    WITH day_data AS (
      SELECT
        dr.id,
        TO_CHAR(dr.report_date, 'YYYY-MM-DD')  AS rdate,
        dr.revision_no,
        dr.status,
        dr.weather->>'condition'               AS weather_cond,
        dr.temperature_min,
        dr.temperature_max,
        dr.notes,
        COALESCE((SELECT SUM(dm.headcount) FROM daily_manpower dm WHERE dm.daily_report_id = dr.id), 0) AS mp_total,
        (SELECT jsonb_agg(jsonb_build_object('trade',dm2.trade,'headcount',dm2.headcount,'subcontractor_id',dm2.subcontractor_id))
           FROM daily_manpower dm2 WHERE dm2.daily_report_id = dr.id) AS mp_rows,
        (SELECT jsonb_agg(jsonb_build_object('equipment_name',de2.equipment_name,'count',de2.count,'working_hours',de2.working_hours))
           FROM daily_equipment de2 WHERE de2.daily_report_id = dr.id) AS eq_rows,
        (SELECT COALESCE(SUM(de3.working_hours),0) FROM daily_equipment de3 WHERE de3.daily_report_id = dr.id) AS eq_hours,
        (SELECT jsonb_agg(jsonb_build_object('description',dwe2.description,'location',dwe2.location,'qty',dwe2.qty,'unit',dwe2.unit))
           FROM daily_work_entries dwe2 WHERE dwe2.daily_report_id = dr.id) AS we_rows,
        (SELECT COUNT(*) FROM daily_work_entries dwe3 WHERE dwe3.daily_report_id = dr.id) AS we_count
      FROM daily_reports dr
      WHERE dr.project_id = wr.project_id
        AND dr.deleted_at IS NULL
        AND dr.report_date BETWEEN wr.period_start AND wr.period_end
        AND dr.revision_no = (
          SELECT MAX(x.revision_no) FROM daily_reports x
          WHERE x.project_id = dr.project_id AND x.report_date = dr.report_date AND x.deleted_at IS NULL
        )
    ),
    proj AS (SELECT name, code FROM projects WHERE id = wr.project_id)
    SELECT jsonb_build_object(
      'generated_at',      NOW(),
      'project_name',      (SELECT name FROM proj),
      'project_code',      (SELECT code FROM proj),
      'week_no',           wr.week_no,
      'period_start',      TO_CHAR(wr.period_start,'YYYY-MM-DD'),
      'period_end',        TO_CHAR(wr.period_end,  'YYYY-MM-DD'),
      'days', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
          'date',            d.rdate,
          'revision_no',     d.revision_no,
          'status',          d.status,
          'weather_condition', d.weather_cond,
          'temperature_min', d.temperature_min,
          'temperature_max', d.temperature_max,
          'notes',           d.notes,
          'manpower_total',  d.mp_total,
          'manpower',        COALESCE(d.mp_rows, '[]'::jsonb),
          'equipment',       COALESCE(d.eq_rows, '[]'::jsonb),
          'work_entries',    COALESCE(d.we_rows, '[]'::jsonb)
        ) ORDER BY d.rdate)
        FROM day_data d
      ), '[]'::jsonb),
      'totals', jsonb_build_object(
        'days_reported',        (SELECT COUNT(*)          FROM day_data),
        'manpower_person_days', (SELECT COALESCE(SUM(mp_total),0) FROM day_data),
        'equipment_hours',      (SELECT COALESCE(SUM(eq_hours),0) FROM day_data),
        'work_entry_count',     (SELECT COALESCE(SUM(we_count),0) FROM day_data)
      ),
      'deliveries',       '[]'::jsonb,
      'stock',            '[]'::jsonb,
      'pending_payments', '[]'::jsonb,
      'pending_pos',      '[]'::jsonb,
      'open_tasks',       0,
      'tasks_due_this_week', 0,
      'pending_mars',     0,
      'ohs_note',         'İSG özeti Faz 8 ile eklenecektir.'
    )
  )
  WHERE wr.project_id = v_proj
    AND wr.deleted_at IS NULL
    AND wr.week_no IN (31, 32);

END $$;
