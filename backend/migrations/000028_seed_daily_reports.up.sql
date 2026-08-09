-- Migration 000028 — Demo seed: günlük saha raporları (Hafta 31–32 / 2026)
-- İdempotent (ON CONFLICT DO NOTHING). DEMO-04 projesinin 2026-07-27..2026-08-09
-- aralığı için 14 günlük rapor, personel, ekipman ve imalat kalemi ekler.
-- Mevcut boş haftalık rapor snapshot'larını yeniden oluşturur.

DO $$
DECLARE
  v_proj   uuid;
  v_admin  uuid;
  dr_id    uuid;

  -- Hafta 31 günlük rapor ID'leri
  dr_0727 uuid := gen_random_uuid();
  dr_0728 uuid := gen_random_uuid();
  dr_0729 uuid := gen_random_uuid();
  dr_0730 uuid := gen_random_uuid();
  dr_0731 uuid := gen_random_uuid();
  dr_0801 uuid := gen_random_uuid();
  dr_0802 uuid := gen_random_uuid();
  -- Hafta 32 günlük rapor ID'leri
  dr_0803 uuid := gen_random_uuid();
  dr_0804 uuid := gen_random_uuid();
  dr_0805 uuid := gen_random_uuid();
  dr_0806 uuid := gen_random_uuid();
  dr_0807 uuid := gen_random_uuid();
  dr_0808 uuid := gen_random_uuid();
  dr_0809 uuid := gen_random_uuid();

BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  SELECT id INTO v_proj  FROM projects WHERE code = 'DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NULL OR v_admin IS NULL THEN RETURN; END IF;

  -- ── Günlük raporları ekle (ON CONFLICT DO NOTHING → idempotent) ──────────

  INSERT INTO daily_reports (id, project_id, report_date, revision_no, status, author_id,
    weather, temperature_min, temperature_max, notes)
  VALUES
  -- Hafta 31: 27 Tem – 2 Ağu 2026
  (dr_0727, v_proj, '2026-07-27', 1, 'Submitted', v_admin,
   '{"condition":"Parçalı Bulutlu"}', 24, 33,
   'Bodrum kat kalıp sökme işlemleri tamamlandı. Beton dökümüne hazırlık yapıldı.'),
  (dr_0728, v_proj, '2026-07-28', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 26, 36,
   'B1 aksı kolonları beton dökümü yapıldı. 4 adet kolon tamamlandı.'),
  (dr_0729, v_proj, '2026-07-29', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 27, 37,
   'B2–B3 aksı kolon kalıpları kuruldu. Demir imalatı devam ediyor.'),
  (dr_0730, v_proj, '2026-07-30', 1, 'Submitted', v_admin,
   '{"condition":"Parçalı Bulutlu"}', 25, 34,
   'Zemin kat döşeme betonarme imalatı: 3 aks tamamlandı.'),
  (dr_0731, v_proj, '2026-07-31', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 26, 35,
   '1. kat perde beton kalıp montajı. Elektrik tesisatı kanalları açıldı.'),
  (dr_0801, v_proj, '2026-08-01', 1, 'Submitted', v_admin,
   '{"condition":"Yağmurlu"}', 19, 26,
   'Hava koşulları nedeniyle beton dökümü yapılamadı. İç mekan imalatlarına devam edildi.'),
  (dr_0802, v_proj, '2026-08-02', 1, 'Submitted', v_admin,
   '{"condition":"Parçalı Bulutlu"}', 22, 30,
   'A aksı 1. kat kolon beton dökümü tamamlandı. Haftalık kontrol yapıldı.'),
  -- Hafta 32: 3–9 Ağu 2026
  (dr_0803, v_proj, '2026-08-03', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 25, 35,
   '1. kat kirişler ve döşeme kalıpları kuruldu. 42 işçi sahada.'),
  (dr_0804, v_proj, '2026-08-04', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 26, 36,
   'C aksı döşeme demiri bağlandı. Alt kat perde duvar sıva başlandı.'),
  (dr_0805, v_proj, '2026-08-05', 1, 'Submitted', v_admin,
   '{"condition":"Parçalı Bulutlu"}', 24, 33,
   '1. kat plak döşeme beton dökümü: 180 m². Vibrasyon işlemi tamamlandı.'),
  (dr_0806, v_proj, '2026-08-06', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 27, 38,
   '2. kat kalıp çalışması başlandı. Elektrik ana dağıtım panosu montajı yapıldı.'),
  (dr_0807, v_proj, '2026-08-07', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 28, 39,
   'Mekanik tesisat (yangın söndürme) boru hatları B bloğa çekildi.'),
  (dr_0808, v_proj, '2026-08-08', 1, 'Submitted', v_admin,
   '{"condition":"Parçalı Bulutlu"}', 25, 34,
   '2. kat D aksı kolon kalıpları tamamlandı. Su yalıtımı çalışmaları sürdü.'),
  (dr_0809, v_proj, '2026-08-09', 1, 'Submitted', v_admin,
   '{"condition":"Açık ve Güneşli"}', 26, 35,
   'Hafta kapanış kontrol ve puantaj. Güvenlik toplantısı yapıldı.')
  ON CONFLICT (project_id, report_date, revision_no) DO NOTHING;

  -- ── Personel (daily_manpower) ─────────────────────────────────────────────
  -- Her günün ID'sini dinamik olarak al (ON CONFLICT sonrası da çalışır)

  FOR dr_id IN
    SELECT id FROM daily_reports
    WHERE project_id = v_proj
      AND report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND revision_no = 1
  LOOP
    INSERT INTO daily_manpower (daily_report_id, trade, headcount, subcontractor_name)
    VALUES
      (dr_id, 'Betonarme', 18, 'Marmara Kaba Yapı ve İnş. A.Ş.'),
      (dr_id, 'Kalıpçı',   10, 'Marmara Kaba Yapı ve İnş. A.Ş.'),
      (dr_id, 'Demirci',    8, 'Marmara Kaba Yapı ve İnş. A.Ş.'),
      (dr_id, 'Elektrikçi', 5, 'Bursa Güç Elektrik Taahhüt Ltd.'),
      (dr_id, 'Tesisatçı',  4, NULL),
      (dr_id, 'İşçi (Genel)', 6, NULL)
    ON CONFLICT DO NOTHING;
  END LOOP;

  -- ── Ekipman (daily_equipment) ─────────────────────────────────────────────
  FOR dr_id IN
    SELECT id FROM daily_reports
    WHERE project_id = v_proj
      AND report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND revision_no = 1
  LOOP
    INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours)
    VALUES
      (dr_id, 'Beton Pompası',  1, 6.0),
      (dr_id, 'Transmikser',    3, 8.0),
      (dr_id, 'Tower Crane',    1, 9.0),
      (dr_id, 'Kompresör',      1, 4.5),
      (dr_id, 'Forklift',       1, 5.0)
    ON CONFLICT DO NOTHING;
  END LOOP;

  -- ── İmalat Kalemleri (daily_work_entries) ────────────────────────────────
  FOR dr_id IN
    SELECT id FROM daily_reports
    WHERE project_id = v_proj
      AND report_date BETWEEN '2026-07-27' AND '2026-08-09'
      AND revision_no = 1
  LOOP
    INSERT INTO daily_work_entries (daily_report_id, description, location, qty, unit)
    VALUES
      (dr_id, 'Kolon Beton Dökümü (C25/30)',          'B Blok – 1. Kat',   12.5, 'm³'),
      (dr_id, 'Döşeme Kalıp Montajı',                  'A Blok – 1. Kat',  180.0, 'm²'),
      (dr_id, 'Betonarme Demir İmalatı (B420C)',       'B Blok',           1850.0, 'kg'),
      (dr_id, 'Perde Duvar Sıva (1. Kat)',             'A Blok İç Mekan',   45.0, 'm²'),
      (dr_id, 'Yangın Söndürme Boru Tesisatı (DN50)',  'B Blok Bodrum',     28.0, 'm')
    ON CONFLICT DO NOTHING;
  END LOOP;

  -- ── Mevcut boş haftalık rapor snapshot'larını yeniden oluştur ────────────
  -- Sadece bu iki haftayı etkiler; snapshot JSONB güncellenir.

  UPDATE weekly_reports wr
  SET snapshot = (
    SELECT jsonb_build_object(
      'generated_at',  NOW(),
      'project_name',  p.name,
      'project_code',  p.code,
      'week_no',       wr.week_no,
      'period_start',  TO_CHAR(wr.period_start, 'YYYY-MM-DD'),
      'period_end',    TO_CHAR(wr.period_end,   'YYYY-MM-DD'),
      'totals', jsonb_build_object(
        'days_reported',        COUNT(DISTINCT dr.id),
        'manpower_person_days', COALESCE(SUM(dm.headcount), 0),
        'equipment_hours',      COALESCE(SUM(de.working_hours), 0),
        'work_entry_count',     COUNT(DISTINCT dwe.id)
      ),
      'days', COALESCE((
        SELECT jsonb_agg(
          jsonb_build_object(
            'date',            TO_CHAR(dr2.report_date, 'YYYY-MM-DD'),
            'revision_no',     dr2.revision_no,
            'status',          dr2.status,
            'weather_condition', (dr2.weather->>'condition'),
            'temperature_min', dr2.temperature_min,
            'temperature_max', dr2.temperature_max,
            'notes',           dr2.notes,
            'manpower_total',  (SELECT COALESCE(SUM(dm2.headcount),0) FROM daily_manpower dm2 WHERE dm2.daily_report_id = dr2.id),
            'manpower', (
              SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'trade', dm3.trade, 'headcount', dm3.headcount, 'subcontractor_name', dm3.subcontractor_name
              )), '[]'::jsonb)
              FROM daily_manpower dm3 WHERE dm3.daily_report_id = dr2.id
            ),
            'equipment', (
              SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'equipment_name', de3.equipment_name, 'count', de3.count, 'working_hours', de3.working_hours
              )), '[]'::jsonb)
              FROM daily_equipment de3 WHERE de3.daily_report_id = dr2.id
            ),
            'work_entries', (
              SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'description', dwe3.description, 'location', dwe3.location, 'qty', dwe3.qty, 'unit', dwe3.unit
              )), '[]'::jsonb)
              FROM daily_work_entries dwe3 WHERE dwe3.daily_report_id = dr2.id
            )
          ) ORDER BY dr2.report_date
        )
        FROM daily_reports dr2
        WHERE dr2.project_id = wr.project_id
          AND dr2.deleted_at IS NULL
          AND dr2.report_date BETWEEN wr.period_start AND wr.period_end
          AND dr2.revision_no = (
            SELECT MAX(x.revision_no) FROM daily_reports x
            WHERE x.project_id = dr2.project_id AND x.report_date = dr2.report_date AND x.deleted_at IS NULL
          )
      ), '[]'::jsonb),
      'deliveries',       '[]'::jsonb,
      'stock',            '[]'::jsonb,
      'pending_payments', '[]'::jsonb,
      'pending_pos',      '[]'::jsonb,
      'open_tasks',       0,
      'tasks_due_this_week', 0,
      'pending_mars',     0,
      'ohs_note',         'İSG özeti Faz 8 ile eklenecektir.'
    )
    FROM projects p
    LEFT JOIN daily_reports dr    ON dr.project_id = wr.project_id AND dr.deleted_at IS NULL
                                   AND dr.report_date BETWEEN wr.period_start AND wr.period_end
    LEFT JOIN daily_manpower dm   ON dm.daily_report_id = dr.id
    LEFT JOIN daily_equipment de  ON de.daily_report_id = dr.id
    LEFT JOIN daily_work_entries dwe ON dwe.daily_report_id = dr.id
    WHERE p.id = wr.project_id
    GROUP BY p.name, p.code
  )
  WHERE wr.project_id = v_proj
    AND wr.deleted_at IS NULL
    AND wr.week_no IN (31, 32)
    AND (wr.snapshot->>'days') = '[]';

END $$;
