-- Faz 3.4 (E16) — günlük rapora yapılandırılmış gecikme nedeni.
-- Her eleman: {"reason_code": "Hava|Sahip|Tasarim|Taseron|Malzeme|Ekipman|Idari|Diger",
--              "hours": 0.5..24, "note": "isteğe bağlı, en fazla 500 karakter"}
ALTER TABLE daily_reports
    ADD COLUMN delay_causes jsonb NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(delay_causes) = 'array');
