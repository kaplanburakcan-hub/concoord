-- Migration 000029 — Disiplin alanları: work entries + haftalık raporlar

-- 1. daily_work_entries.discipline
ALTER TABLE daily_work_entries ADD COLUMN IF NOT EXISTS discipline TEXT;

-- 2. weekly_reports.next_week_plans
ALTER TABLE weekly_reports
  ADD COLUMN IF NOT EXISTS next_week_plans JSONB NOT NULL DEFAULT '[]'::jsonb;

-- 3. DEMO-04 work entries: disiplin ata
DO $$
DECLARE v_proj uuid;
BEGIN
  SELECT id INTO v_proj FROM projects WHERE code='DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NULL THEN RETURN; END IF;

  UPDATE daily_work_entries dwe
  SET discipline = CASE
    WHEN dwe.description ILIKE '%yangın%'
      OR dwe.description ILIKE '%boru%'
      OR dwe.description ILIKE '%tesisat%'
      OR dwe.description ILIKE '%sprinkler%'
      OR dwe.description ILIKE '%hvac%'
      OR dwe.description ILIKE '%mekanik%' THEN 'Mekanik'
    WHEN dwe.description ILIKE '%kablo%'
      OR dwe.description ILIKE '%elektrik%'
      OR dwe.description ILIKE '%pano%'
      OR dwe.description ILIKE '%trafo%' THEN 'Elektrik'
    ELSE 'İnşaat'
  END
  FROM daily_reports dr
  WHERE dwe.daily_report_id = dr.id
    AND dr.project_id = v_proj
    AND dr.deleted_at IS NULL
    AND dr.status != 'Submitted'
    AND dwe.discipline IS NULL;

  -- 4. Hafta 31 snapshot → discipline_sections ekle
  UPDATE weekly_reports
  SET snapshot = snapshot || '{
    "discipline_sections": [
      {
        "discipline": "İnşaat",
        "this_week": [
          "Kolon Beton Dökümü (C25/30) — B Blok – 1. Kat (87.5 m³ toplam)",
          "Döşeme Kalıp Montajı — A Blok – 1. Kat (1260.0 m² toplam)",
          "Betonarme Demir İmalatı (B420C) — B Blok (12950.0 kg toplam)",
          "Perde Duvar Sıva (1. Kat) — A Blok İç Mekan (315.0 m² toplam)"
        ]
      },
      {
        "discipline": "Mekanik",
        "this_week": [
          "Yangın Söndürme Boru Tesisatı (DN50) — B Blok Bodrum (196.0 m toplam)"
        ]
      }
    ]
  }'::jsonb,
  next_week_plans = '[
    {"discipline":"İnşaat","plans":["A Blok 1. kat kolon betonu döküm hazırlığı","B Blok kirişler ve döşeme kalıpları montajı","C aksı döşeme demiri bağlama"]},
    {"discipline":"Mekanik","plans":["B Blok sprinkler boru montajı (1. kat)","A Blok sıhhi tesisat ana hatları çekimi"]}
  ]'::jsonb
  WHERE project_id = v_proj
    AND week_no = 31
    AND deleted_at IS NULL;

  -- 5. Hafta 32 snapshot → discipline_sections ekle
  UPDATE weekly_reports
  SET snapshot = snapshot || '{
    "discipline_sections": [
      {
        "discipline": "İnşaat",
        "this_week": [
          "Kolon Beton Dökümü (C25/30) — B Blok – 1. Kat (75.0 m³ toplam)",
          "Döşeme Kalıp Montajı — A Blok – 1. Kat (1080.0 m² toplam)",
          "Betonarme Demir İmalatı (B420C) — B Blok (11100.0 kg toplam)",
          "Perde Duvar Sıva (1. Kat) — A Blok İç Mekan (270.0 m² toplam)"
        ]
      },
      {
        "discipline": "Mekanik",
        "this_week": [
          "Yangın Söndürme Boru Tesisatı (DN50) — B Blok Bodrum (168.0 m toplam)"
        ]
      }
    ]
  }'::jsonb,
  next_week_plans = '[
    {"discipline":"İnşaat","plans":["2. kat kolon betonu döküm hazırlığı","B Blok dış cephe hazırlık çalışmaları","C Blok zemin kat kalıp montajı"]},
    {"discipline":"Mekanik","plans":["B Blok HVAC ana kanal bağlantıları","Yangın algılama kablo tesisatı (B Blok 1-2. kat)"]}
  ]'::jsonb
  WHERE project_id = v_proj
    AND week_no = 32
    AND deleted_at IS NULL;

END $$;
