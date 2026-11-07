-- ============================================================================
-- DEMO-05 — Sakarya Entegre Lojistik Depo ve İdari Bina (gen_demo05.py çıktısı)
-- Fiziki ilerleme ~%60, finansal ilerleme ~%65, CPI ~0.92, SPI ~0.96 hedeflenir.
-- Önceden DEMO-05 varsa arşivlenir (idempotent).
-- ============================================================================
DO $$
DECLARE
  v_admin uuid; v_role uuid; v_proj uuid;
BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  IF v_admin IS NULL THEN RAISE EXCEPTION 'Kullanıcı yok'; END IF;
  SELECT id INTO v_role FROM roles WHERE name = 'ProjectManager' LIMIT 1;
  IF v_role IS NULL THEN SELECT id INTO v_role FROM roles ORDER BY created_at LIMIT 1; END IF;
  UPDATE projects SET code = code || '-ARSIV-' || to_char(now(),'YYYYMMDDHH24MISS'), name = name || ' (arşiv)', status = 'Archived', deleted_at = now()
  WHERE code = 'DEMO-05' AND deleted_at IS NULL;

  -- 1) PROJE
  INSERT INTO projects (code, name, location, client_name, budget_total, currency, start_date, end_date, status)
  VALUES ('DEMO-05', 'Sakarya Entegre Lojistik Depo ve İdari Bina', 'Sakarya / Hendek',
          'Sakarya Lojistik Yatırım A.Ş.', 180000000.00, 'TRY', DATE '2025-06-01', DATE '2027-12-31', 'Active')
  RETURNING id INTO v_proj;
  INSERT INTO project_members (project_id, user_id, role_id) VALUES (v_proj, v_admin, v_role);

  -- 2) ANA SÖZLEŞME (idari hakediş oranı bu bedele göre)
  INSERT INTO project_main_contracts (project_id, sozlesme_turu, fiyat_farki_var, sozlesme_bedeli, sozlesme_para_birimi,
    sozlesme_tarihi, yer_teslim_tarihi, is_suresi_gun, is_turu, kdv_pct, isveren_adi, yuklenici_adi, yuklenici_proje_sorumlusu,
    created_by, updated_by, is_locked)
  VALUES (v_proj, 'goturu_bedel', false, 180000000.00, 'TRY', DATE '2025-05-15', DATE '2025-06-01', 915, 'yapim', 20,
    'Sakarya Lojistik Yatırım A.Ş.', 'Demo Yapı Müteahhitlik A.Ş.', 'Proje Müdürü (Demo)', v_admin, v_admin, false);

  -- 3) TAŞERONLAR + SÖZLEŞMELER + BİRİM FİYAT POZLARI
INSERT INTO subcontractors (id, project_id, company_name, tax_no, contact_person, phone, email, trade)
VALUES ('e46ecd2e-7006-4c6a-89b0-36bc1634aab4', v_proj, 'Güçlü Toprak ve Hafriyat A.Ş.', '5101112233', 'Hasan Çelik', '0537 100 1100', 'proje@guclutoprak.com', 'Toprak İşleri');
INSERT INTO contracts (id, project_id, subcontractor_id, contract_no, type, amount, advance_amount, retention_pct,
    advance_rate_pct, sign_date, start_date, end_date, is_multi_year, withholding_pct)
VALUES ('9ac05135-3997-4723-9936-0346f2e8355d', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 'DEMO05-SZL-01', 'Sub', 21900000.00, 3285000.00, 5.00, 15.00,
    DATE '2025-07-01', DATE '2025-07-01', DATE '2027-06-30', true, 0.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('596bebc0-a319-49de-8873-cd4495272e41', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', '9ac05135-3997-4723-9936-0346f2e8355d', 'Y.16.001', 'Kazı ve hafriyat taşınması', 'm3', 150000.00, 90.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('073880c0-c1c4-4244-a005-191f1380c8f1', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', '9ac05135-3997-4723-9936-0346f2e8355d', 'Y.16.080', 'Granüler dolgu serimi', 'm3', 60000.00, 140.00);
INSERT INTO subcontractors (id, project_id, company_name, tax_no, contact_person, phone, email, trade)
VALUES ('ccc92ec6-fe50-4b95-8635-0c1aff517b46', v_proj, 'Karadeniz Beton Yapı Ltd.', '5102223344', 'Fatma Arslan', '0532 200 2200', 'info@karadenizbeton.com', 'Betonarme');
INSERT INTO contracts (id, project_id, subcontractor_id, contract_no, type, amount, advance_amount, retention_pct,
    advance_rate_pct, sign_date, start_date, end_date, is_multi_year, withholding_pct)
VALUES ('85adb222-1bec-49a5-96f9-825f224f0a8e', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'DEMO05-SZL-02', 'Sub', 60080000.00, 9012000.00, 5.00, 15.00,
    DATE '2025-10-01', DATE '2025-10-01', DATE '2027-06-30', true, 0.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('4d98d4fb-ae59-4fbc-91af-878bf23898bc', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', '85adb222-1bec-49a5-96f9-825f224f0a8e', 'Y.17.210', 'Temel betonu (C30/37)', 'm3', 6500.00, 2600.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('77b9cf86-f52b-44bd-b955-b88ea6c79c29', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', '85adb222-1bec-49a5-96f9-825f224f0a8e', 'Y.17.240', 'Kolon-kiriş-perde betonu (C35/45)', 'm3', 4200.00, 3200.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('c6c30c32-1190-4f28-9034-93956467475c', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', '85adb222-1bec-49a5-96f9-825f224f0a8e', 'Y.17.260', 'Döşeme betonu (C30/37)', 'm3', 3800.00, 2900.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('15393385-1d43-4ce7-93be-3cde99361431', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', '85adb222-1bec-49a5-96f9-825f224f0a8e', 'Y.17.300', 'Nervürlü betonarme donatı', 'ton', 780.00, 24000.00);
INSERT INTO subcontractors (id, project_id, company_name, tax_no, contact_person, phone, email, trade)
VALUES ('dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', v_proj, 'Anadolu Çelik Konstrüksiyon A.Ş.', '5103334455', 'Kemal Şahin', '0533 300 3300', 'proje@anadolucelik.com', 'Çelik Konstrüksiyon');
INSERT INTO contracts (id, project_id, subcontractor_id, contract_no, type, amount, advance_amount, retention_pct,
    advance_rate_pct, sign_date, start_date, end_date, is_multi_year, withholding_pct)
VALUES ('6bfc645e-89fb-40e7-a846-68b4ca9ee216', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'DEMO05-SZL-03', 'Sub', 33940000.00, 5091000.00, 5.00, 15.00,
    DATE '2026-03-01', DATE '2026-03-01', DATE '2027-06-30', true, 0.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('066125af-cc64-4bad-a483-92b7f637434d', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', '6bfc645e-89fb-40e7-a846-68b4ca9ee216', 'Y.20.110', 'Çelik kolon-kiriş imalat ve montajı', 'ton', 950.00, 26000.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', '6bfc645e-89fb-40e7-a846-68b4ca9ee216', 'Y.20.160', 'Çatı makas ve gerdirme', 'ton', 420.00, 22000.00);
INSERT INTO subcontractors (id, project_id, company_name, tax_no, contact_person, phone, email, trade)
VALUES ('995a0a86-2a04-4d9a-a986-312025cee542', v_proj, 'Işık Mekanik-Elektrik Tes. Ltd.', '5104445566', 'Sibel Güneş', '0534 400 4400', 'info@isikmep.com', 'Mekanik-Elektrik');
INSERT INTO contracts (id, project_id, subcontractor_id, contract_no, type, amount, advance_amount, retention_pct,
    advance_rate_pct, sign_date, start_date, end_date, is_multi_year, withholding_pct)
VALUES ('c924ea87-04de-4207-b762-44c7771c6347', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 'DEMO05-SZL-04', 'Sub', 23690000.00, 3553500.00, 5.00, 15.00,
    DATE '2026-06-01', DATE '2026-06-01', DATE '2027-06-30', true, 0.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 'c924ea87-04de-4207-b762-44c7771c6347', 'Y.25.100', 'Sıhhi tesisat borulama', 'm', 9500.00, 780.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('8430b3e2-233f-4a9c-b271-a7337691bf2f', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 'c924ea87-04de-4207-b762-44c7771c6347', 'Y.26.100', 'Elektrik kablo ve kanal', 'm', 18000.00, 420.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('791aab51-1bab-4b47-8ccc-93f3bd38d7d6', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 'c924ea87-04de-4207-b762-44c7771c6347', 'Y.26.300', 'Aydınlatma armatürü', 'adet', 1200.00, 1850.00);
INSERT INTO work_items (id, project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
VALUES ('7207d5b0-244d-4df4-9027-c6724f84479e', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 'c924ea87-04de-4207-b762-44c7771c6347', 'Y.25.300', 'HVAC klima sistemi', 'ad', 1.00, 6500000.00);

  -- 4) MALİYET KODLARI + BÜTÇE (toplam = BAC)
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('b68ad254-f8a7-46b8-b77d-f432c2622061', v_proj, '01', 'Genel Giderler ve Şantiye', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('b68ad254-f8a7-46b8-b77d-f432c2622061', v_proj, 14000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('470700f9-4df8-4cfa-b558-e260cb491de2', v_proj, '02', 'Hafriyat ve Altyapı', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('470700f9-4df8-4cfa-b558-e260cb491de2', v_proj, 22000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('814b4bd7-c42f-4eae-892a-08232516e9f4', v_proj, '03', 'Kaba İnşaat (Betonarme)', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('814b4bd7-c42f-4eae-892a-08232516e9f4', v_proj, 62000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('35719ad9-39fa-496d-8dce-53ae353d452a', v_proj, '04', 'Çelik Yapı', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('35719ad9-39fa-496d-8dce-53ae353d452a', v_proj, 30000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('a45ec96a-a5ac-4e41-8a32-04a4a5284f93', v_proj, '05', 'Mekanik Tesisat', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('a45ec96a-a5ac-4e41-8a32-04a4a5284f93', v_proj, 18000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('f5316db0-2da3-41e4-a5f9-f8e94ef751fb', v_proj, '06', 'Elektrik Tesisat', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('f5316db0-2da3-41e4-a5f9-f8e94ef751fb', v_proj, 16000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('67b589fb-7045-47be-9d0a-379557334805', v_proj, '07', 'Dış Cephe ve Kaplama', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('67b589fb-7045-47be-9d0a-379557334805', v_proj, 12000000.00, v_admin);
INSERT INTO cost_codes (id, project_id, code, name, created_by) VALUES ('c3fe8cd7-7c3b-44b8-a784-37ed533f4246', v_proj, '08', 'Yol ve Saha', v_admin);
INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by) VALUES ('c3fe8cd7-7c3b-44b8-a784-37ed533f4246', v_proj, 6000000.00, v_admin);

  -- 5) HAKEDİŞLER (kesinleşmiş; EV ve AC buradan türer)
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('ac236af0-6b43-438a-8db2-b2d766be93d6', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 1, DATE '2025-07-01', DATE '2025-07-31', 'Draft',
    751028.10, 0.00, 751028.10, 37551.40, 20.00, 713476.70,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 150205.62, 150205.62, 751028.10, 728497.26, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ac236af0-6b43-438a-8db2-b2d766be93d6', '596bebc0-a319-49de-8873-cd4495272e41', 0.00, 5144.03, 5144.03, 462962.70, 462962.70);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ac236af0-6b43-438a-8db2-b2d766be93d6', '073880c0-c1c4-4244-a005-191f1380c8f1', 0.00, 2057.61, 2057.61, 288065.40, 288065.40);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-08-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-08-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-08-05 11:00:00+03' WHERE id='ac236af0-6b43-438a-8db2-b2d766be93d6';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('12687e56-e770-42e6-a897-aad4420a569f', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 2, DATE '2025-08-01', DATE '2025-08-31', 'Draft',
    2763786.40, 751028.10, 2012758.30, 100637.92, 20.00, 1912120.38,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 402551.66, 402551.66, 2012758.30, 1952375.55, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('12687e56-e770-42e6-a897-aad4420a569f', '596bebc0-a319-49de-8873-cd4495272e41', 5144.03, 13786.01, 18930.04, 1703703.60, 1240740.90);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('12687e56-e770-42e6-a897-aad4420a569f', '073880c0-c1c4-4244-a005-191f1380c8f1', 2057.61, 5514.41, 7572.02, 1060082.80, 772017.40);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-09-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-09-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-09-05 11:00:00+03' WHERE id='12687e56-e770-42e6-a897-aad4420a569f';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('2acd4634-4980-4fa9-b209-5fb1ca9bd8c4', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 3, DATE '2025-09-01', DATE '2025-09-30', 'Draft',
    5677778.50, 2763786.40, 2913992.10, 145699.61, 20.00, 2768292.49,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 582798.42, 582798.42, 2913992.10, 2826572.34, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('2acd4634-4980-4fa9-b209-5fb1ca9bd8c4', '596bebc0-a319-49de-8873-cd4495272e41', 18930.04, 19958.85, 38888.89, 3500000.10, 1796296.50);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('2acd4634-4980-4fa9-b209-5fb1ca9bd8c4', '073880c0-c1c4-4244-a005-191f1380c8f1', 7572.02, 7983.54, 15555.56, 2177778.40, 1117695.60);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-10-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-10-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-10-05 11:00:00+03' WHERE id='2acd4634-4980-4fa9-b209-5fb1ca9bd8c4';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('94bfdea9-db28-4c89-854a-294f7114b553', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 4, DATE '2025-10-01', DATE '2025-10-31', 'Draft',
    9132510.80, 5677778.50, 3454732.30, 172736.61, 20.00, 3281995.69,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 690946.46, 690946.46, 3454732.30, 3351090.33, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('94bfdea9-db28-4c89-854a-294f7114b553', '596bebc0-a319-49de-8873-cd4495272e41', 38888.89, 23662.55, 62551.44, 5629629.60, 2129629.50);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('94bfdea9-db28-4c89-854a-294f7114b553', '073880c0-c1c4-4244-a005-191f1380c8f1', 15555.56, 9465.02, 25020.58, 3502881.20, 1325102.80);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-11-05 11:00:00+03' WHERE id='94bfdea9-db28-4c89-854a-294f7114b553';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('011742ce-e8f8-4550-a882-c6fb1d676419', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 5, DATE '2025-11-01', DATE '2025-11-30', 'Draft',
    12767489.20, 9132510.80, 3634978.40, 181748.92, 20.00, 3453229.48,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 726995.68, 726995.68, 3634978.40, 3525929.05, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('011742ce-e8f8-4550-a882-c6fb1d676419', '596bebc0-a319-49de-8873-cd4495272e41', 62551.44, 24897.12, 87448.56, 7870370.40, 2240740.80);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('011742ce-e8f8-4550-a882-c6fb1d676419', '073880c0-c1c4-4244-a005-191f1380c8f1', 25020.58, 9958.84, 34979.42, 4897118.80, 1394237.60);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-12-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-12-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-12-05 11:00:00+03' WHERE id='011742ce-e8f8-4550-a882-c6fb1d676419';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('a73bbe5a-976d-4e0f-93c7-0bd004b2e8b2', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 6, DATE '2025-12-01', DATE '2025-12-31', 'Draft',
    16222221.50, 12767489.20, 3454732.30, 172736.61, 20.00, 3281995.69,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 690946.46, 690946.46, 3454732.30, 3351090.33, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a73bbe5a-976d-4e0f-93c7-0bd004b2e8b2', '596bebc0-a319-49de-8873-cd4495272e41', 87448.56, 23662.55, 111111.11, 9999999.90, 2129629.50);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a73bbe5a-976d-4e0f-93c7-0bd004b2e8b2', '073880c0-c1c4-4244-a005-191f1380c8f1', 34979.42, 9465.02, 44444.44, 6222221.60, 1325102.80);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-01-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-01-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-01-05 11:00:00+03' WHERE id='a73bbe5a-976d-4e0f-93c7-0bd004b2e8b2';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('54f0a045-b25b-4e1d-b1fb-52f2c5659f6b', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 7, DATE '2026-01-01', DATE '2026-01-31', 'Draft',
    19136213.60, 16222221.50, 2913992.10, 145699.61, 20.00, 2768292.49,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 582798.42, 582798.42, 2913992.10, 2826572.34, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('54f0a045-b25b-4e1d-b1fb-52f2c5659f6b', '596bebc0-a319-49de-8873-cd4495272e41', 111111.11, 19958.85, 131069.96, 11796296.40, 1796296.50);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('54f0a045-b25b-4e1d-b1fb-52f2c5659f6b', '073880c0-c1c4-4244-a005-191f1380c8f1', 44444.44, 7983.54, 52427.98, 7339917.20, 1117695.60);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-02-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-02-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-02-05 11:00:00+03' WHERE id='54f0a045-b25b-4e1d-b1fb-52f2c5659f6b';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('fa3c36f2-0bc3-458c-b3a4-8f7f850f1bb0', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 8, DATE '2026-02-01', DATE '2026-02-28', 'Draft',
    21148971.90, 19136213.60, 2012758.30, 100637.92, 20.00, 1912120.38,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 402551.66, 402551.66, 2012758.30, 1952375.55, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa3c36f2-0bc3-458c-b3a4-8f7f850f1bb0', '596bebc0-a319-49de-8873-cd4495272e41', 131069.96, 13786.01, 144855.97, 13037037.30, 1240740.90);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa3c36f2-0bc3-458c-b3a4-8f7f850f1bb0', '073880c0-c1c4-4244-a005-191f1380c8f1', 52427.98, 5514.41, 57942.39, 8111934.60, 772017.40);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-03-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-03-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-03-05 11:00:00+03' WHERE id='fa3c36f2-0bc3-458c-b3a4-8f7f850f1bb0';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('5c2890f9-5ff5-435f-b647-cd5d62a075a3', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 9, DATE '2026-03-01', DATE '2026-03-31', 'Draft',
    21900000.00, 21148971.90, 751028.10, 37551.40, 20.00, 713476.70,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 150205.62, 150205.62, 751028.10, 728497.26, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('5c2890f9-5ff5-435f-b647-cd5d62a075a3', '596bebc0-a319-49de-8873-cd4495272e41', 144855.97, 5144.03, 150000.00, 13500000.00, 462962.70);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('5c2890f9-5ff5-435f-b647-cd5d62a075a3', '073880c0-c1c4-4244-a005-191f1380c8f1', 57942.39, 2057.61, 60000.00, 8400000.00, 288065.40);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-04-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-04-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-04-05 11:00:00+03' WHERE id='5c2890f9-5ff5-435f-b647-cd5d62a075a3';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('8116d004-6f5c-4c5a-be53-481e8c1b0360', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 10, DATE '2026-04-01', DATE '2026-04-30', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('8116d004-6f5c-4c5a-be53-481e8c1b0360', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('8116d004-6f5c-4c5a-be53-481e8c1b0360', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-05-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-05-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-05-05 11:00:00+03' WHERE id='8116d004-6f5c-4c5a-be53-481e8c1b0360';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('1cd7b2c8-7cff-4436-9899-4e7d5ab8dd16', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 11, DATE '2026-05-01', DATE '2026-05-31', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1cd7b2c8-7cff-4436-9899-4e7d5ab8dd16', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1cd7b2c8-7cff-4436-9899-4e7d5ab8dd16', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-06-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-06-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-06-05 11:00:00+03' WHERE id='1cd7b2c8-7cff-4436-9899-4e7d5ab8dd16';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('58bcaafe-92c7-424c-b31e-a9014ee840af', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 12, DATE '2026-06-01', DATE '2026-06-30', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('58bcaafe-92c7-424c-b31e-a9014ee840af', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('58bcaafe-92c7-424c-b31e-a9014ee840af', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-07-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-07-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-07-05 11:00:00+03' WHERE id='58bcaafe-92c7-424c-b31e-a9014ee840af';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('d2d8b88f-1b5c-4f99-8448-7ddbcc7dfbd6', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 13, DATE '2026-07-01', DATE '2026-07-31', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('d2d8b88f-1b5c-4f99-8448-7ddbcc7dfbd6', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('d2d8b88f-1b5c-4f99-8448-7ddbcc7dfbd6', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-08-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-08-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-08-05 11:00:00+03' WHERE id='d2d8b88f-1b5c-4f99-8448-7ddbcc7dfbd6';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('38dc5113-7466-409b-9bba-de2ad1b08949', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 14, DATE '2026-08-01', DATE '2026-08-31', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('38dc5113-7466-409b-9bba-de2ad1b08949', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('38dc5113-7466-409b-9bba-de2ad1b08949', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-09-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-09-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-09-05 11:00:00+03' WHERE id='38dc5113-7466-409b-9bba-de2ad1b08949';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('4e569a58-78af-48cd-8a5f-8f702eda634d', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 15, DATE '2026-09-01', DATE '2026-09-30', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('4e569a58-78af-48cd-8a5f-8f702eda634d', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('4e569a58-78af-48cd-8a5f-8f702eda634d', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-10-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-10-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-10-05 11:00:00+03' WHERE id='4e569a58-78af-48cd-8a5f-8f702eda634d';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('4550f4b3-eba9-454e-bf1c-ac8220bcafd2', v_proj, 'e46ecd2e-7006-4c6a-89b0-36bc1634aab4', 16, DATE '2026-10-01', DATE '2026-10-31', 'Draft',
    21900000.00, 21900000.00, 0.00, 0.00, 20.00, 0.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 0.00, 0.00, 0.00, 0.00, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('4550f4b3-eba9-454e-bf1c-ac8220bcafd2', '596bebc0-a319-49de-8873-cd4495272e41', 150000.00, 0.00, 150000.00, 13500000.00, 0.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('4550f4b3-eba9-454e-bf1c-ac8220bcafd2', '073880c0-c1c4-4244-a005-191f1380c8f1', 60000.00, 0.00, 60000.00, 8400000.00, 0.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-11-05 11:00:00+03' WHERE id='4550f4b3-eba9-454e-bf1c-ac8220bcafd2';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('51b8788e-cadf-4f94-8741-9b2f0b76d65f', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 1, DATE '2025-10-01', DATE '2025-10-31', 'Draft',
    761948.00, 0.00, 761948.00, 38097.40, 20.00, 723850.60,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 152389.60, 152389.60, 761948.00, 739089.56, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('51b8788e-cadf-4f94-8741-9b2f0b76d65f', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 0.00, 82.44, 82.44, 214344.00, 214344.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('51b8788e-cadf-4f94-8741-9b2f0b76d65f', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 0.00, 53.27, 53.27, 170464.00, 170464.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('51b8788e-cadf-4f94-8741-9b2f0b76d65f', 'c6c30c32-1190-4f28-9034-93956467475c', 0.00, 48.20, 48.20, 139780.00, 139780.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('51b8788e-cadf-4f94-8741-9b2f0b76d65f', '15393385-1d43-4ce7-93be-3cde99361431', 0.00, 9.89, 9.89, 237360.00, 237360.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-11-05 11:00:00+03' WHERE id='51b8788e-cadf-4f94-8741-9b2f0b76d65f';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('01229cf4-5366-470c-904d-a58260f74350', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 2, DATE '2025-11-01', DATE '2025-11-30', 'Draft',
    2883287.00, 761948.00, 2121339.00, 106066.95, 20.00, 2015272.05,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 424267.80, 424267.80, 2121339.00, 2057698.83, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('01229cf4-5366-470c-904d-a58260f74350', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 82.44, 229.51, 311.95, 811070.00, 596726.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('01229cf4-5366-470c-904d-a58260f74350', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 53.27, 148.30, 201.57, 645024.00, 474560.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('01229cf4-5366-470c-904d-a58260f74350', 'c6c30c32-1190-4f28-9034-93956467475c', 48.20, 134.17, 182.37, 528873.00, 389093.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('01229cf4-5366-470c-904d-a58260f74350', '15393385-1d43-4ce7-93be-3cde99361431', 9.89, 27.54, 37.43, 898320.00, 660960.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2025-12-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2025-12-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2025-12-05 11:00:00+03' WHERE id='01229cf4-5366-470c-904d-a58260f74350';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('cdc09eac-f5c1-479c-ab69-4da3ceb50099', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 3, DATE '2025-12-01', DATE '2025-12-31', 'Draft',
    6116859.00, 2883287.00, 3233572.00, 161678.60, 20.00, 3071893.40,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 646714.40, 646714.40, 3233572.00, 3136564.84, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('cdc09eac-f5c1-479c-ab69-4da3ceb50099', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 311.95, 349.84, 661.79, 1720654.00, 909584.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('cdc09eac-f5c1-479c-ab69-4da3ceb50099', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 201.57, 226.05, 427.62, 1368384.00, 723360.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('cdc09eac-f5c1-479c-ab69-4da3ceb50099', 'c6c30c32-1190-4f28-9034-93956467475c', 182.37, 204.52, 386.89, 1121981.00, 593108.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('cdc09eac-f5c1-479c-ab69-4da3ceb50099', '15393385-1d43-4ce7-93be-3cde99361431', 37.43, 41.98, 79.41, 1905840.00, 1007520.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-01-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-01-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-01-05 11:00:00+03' WHERE id='cdc09eac-f5c1-479c-ab69-4da3ceb50099';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('455f0b32-4571-4bf5-bbbe-e376cbdec220', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 4, DATE '2026-01-01', DATE '2026-01-31', 'Draft',
    10215390.00, 6116859.00, 4098531.00, 204926.55, 20.00, 3893604.45,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 819706.20, 819706.20, 4098531.00, 3975575.07, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('455f0b32-4571-4bf5-bbbe-e376cbdec220', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 661.79, 443.42, 1105.21, 2873546.00, 1152892.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('455f0b32-4571-4bf5-bbbe-e376cbdec220', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 427.62, 286.51, 714.13, 2285216.00, 916832.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('455f0b32-4571-4bf5-bbbe-e376cbdec220', 'c6c30c32-1190-4f28-9034-93956467475c', 386.89, 259.23, 646.12, 1873748.00, 751767.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('455f0b32-4571-4bf5-bbbe-e376cbdec220', '15393385-1d43-4ce7-93be-3cde99361431', 79.41, 53.21, 132.62, 3182880.00, 1277040.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-02-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-02-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-02-05 11:00:00+03' WHERE id='455f0b32-4571-4bf5-bbbe-e376cbdec220';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('96e70d9a-2921-4f65-8fd8-eda1c4924ee1', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 5, DATE '2026-02-01', DATE '2026-02-28', 'Draft',
    14932055.00, 10215390.00, 4716665.00, 235833.25, 20.00, 4480831.75,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 943333.00, 943333.00, 4716665.00, 4575165.05, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('96e70d9a-2921-4f65-8fd8-eda1c4924ee1', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 1105.21, 510.27, 1615.48, 4200248.00, 1326702.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('96e70d9a-2921-4f65-8fd8-eda1c4924ee1', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 714.13, 329.72, 1043.85, 3340320.00, 1055104.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('96e70d9a-2921-4f65-8fd8-eda1c4924ee1', 'c6c30c32-1190-4f28-9034-93956467475c', 646.12, 298.31, 944.43, 2738847.00, 865099.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('96e70d9a-2921-4f65-8fd8-eda1c4924ee1', '15393385-1d43-4ce7-93be-3cde99361431', 132.62, 61.24, 193.86, 4652640.00, 1469760.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-03-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-03-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-03-05 11:00:00+03' WHERE id='96e70d9a-2921-4f65-8fd8-eda1c4924ee1';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('f00f4dec-e6c6-437f-89a7-7b9d388980f4', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 6, DATE '2026-03-01', DATE '2026-03-31', 'Draft',
    20019065.00, 14932055.00, 5087010.00, 254350.50, 20.00, 4832659.50,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 1017402.00, 1017402.00, 5087010.00, 4934399.70, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('f00f4dec-e6c6-437f-89a7-7b9d388980f4', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 1615.48, 550.37, 2165.85, 5631210.00, 1430962.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('f00f4dec-e6c6-437f-89a7-7b9d388980f4', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 1043.85, 355.62, 1399.47, 4478304.00, 1137984.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('f00f4dec-e6c6-437f-89a7-7b9d388980f4', 'c6c30c32-1190-4f28-9034-93956467475c', 944.43, 321.76, 1266.19, 3671951.00, 933104.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('f00f4dec-e6c6-437f-89a7-7b9d388980f4', '15393385-1d43-4ce7-93be-3cde99361431', 193.86, 66.04, 259.90, 6237600.00, 1584960.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-04-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-04-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-04-05 11:00:00+03' WHERE id='f00f4dec-e6c6-437f-89a7-7b9d388980f4';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 7, DATE '2026-04-01', DATE '2026-04-30', 'Draft',
    25229832.00, 20019065.00, 5210767.00, 260538.35, 20.00, 4950228.65,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 1042153.40, 1042153.40, 5210767.00, 5054443.99, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 2165.85, 563.75, 2729.60, 7096960.00, 1465750.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 1399.47, 364.27, 1763.74, 5643968.00, 1165664.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4', 'c6c30c32-1190-4f28-9034-93956467475c', 1266.19, 329.57, 1595.76, 4627704.00, 955753.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4', '15393385-1d43-4ce7-93be-3cde99361431', 259.90, 67.65, 327.55, 7861200.00, 1623600.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-05-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-05-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-05-05 11:00:00+03' WHERE id='c9aef23b-bf41-4b7a-a702-5d7d1bb47dd4';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('198344c0-9137-437f-944c-50ede15d320d', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 8, DATE '2026-05-01', DATE '2026-05-31', 'Draft',
    30317114.00, 25229832.00, 5087282.00, 254364.10, 20.00, 4832917.90,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 1017456.40, 1017456.40, 5087282.00, 4934663.54, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('198344c0-9137-437f-944c-50ede15d320d', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 2729.60, 550.37, 3279.97, 8527922.00, 1430962.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('198344c0-9137-437f-944c-50ede15d320d', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 1763.74, 355.63, 2119.37, 6781984.00, 1138016.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('198344c0-9137-437f-944c-50ede15d320d', 'c6c30c32-1190-4f28-9034-93956467475c', 1595.76, 321.76, 1917.52, 5560808.00, 933104.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('198344c0-9137-437f-944c-50ede15d320d', '15393385-1d43-4ce7-93be-3cde99361431', 327.55, 66.05, 393.60, 9446400.00, 1585200.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-06-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-06-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-06-05 11:00:00+03' WHERE id='198344c0-9137-437f-944c-50ede15d320d';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('80a9ea10-6f78-49a8-afc4-29e3ba2dad6c', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 9, DATE '2026-06-01', DATE '2026-06-30', 'Draft',
    35033507.00, 30317114.00, 4716393.00, 235819.65, 20.00, 4480573.35,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 943278.60, 943278.60, 4716393.00, 4574901.21, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('80a9ea10-6f78-49a8-afc4-29e3ba2dad6c', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 3279.97, 510.27, 3790.24, 9854624.00, 1326702.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('80a9ea10-6f78-49a8-afc4-29e3ba2dad6c', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 2119.37, 329.71, 2449.08, 7837056.00, 1055072.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('80a9ea10-6f78-49a8-afc4-29e3ba2dad6c', 'c6c30c32-1190-4f28-9034-93956467475c', 1917.52, 298.31, 2215.83, 6425907.00, 865099.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('80a9ea10-6f78-49a8-afc4-29e3ba2dad6c', '15393385-1d43-4ce7-93be-3cde99361431', 393.60, 61.23, 454.83, 10915920.00, 1469520.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-07-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-07-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-07-05 11:00:00+03' WHERE id='80a9ea10-6f78-49a8-afc4-29e3ba2dad6c';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('b0b6154f-6e81-4afc-9285-5ef54fcafb45', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 10, DATE '2026-07-01', DATE '2026-07-31', 'Draft',
    39132070.00, 35033507.00, 4098563.00, 204928.15, 20.00, 3893634.85,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 819712.60, 819712.60, 4098563.00, 3975606.11, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b0b6154f-6e81-4afc-9285-5ef54fcafb45', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 3790.24, 443.42, 4233.66, 11007516.00, 1152892.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b0b6154f-6e81-4afc-9285-5ef54fcafb45', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 2449.08, 286.52, 2735.60, 8753920.00, 916864.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b0b6154f-6e81-4afc-9285-5ef54fcafb45', 'c6c30c32-1190-4f28-9034-93956467475c', 2215.83, 259.23, 2475.06, 7177674.00, 751767.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b0b6154f-6e81-4afc-9285-5ef54fcafb45', '15393385-1d43-4ce7-93be-3cde99361431', 454.83, 53.21, 508.04, 12192960.00, 1277040.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-08-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-08-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-08-05 11:00:00+03' WHERE id='b0b6154f-6e81-4afc-9285-5ef54fcafb45';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 11, DATE '2026-08-01', DATE '2026-08-31', 'Draft',
    42365584.00, 39132070.00, 3233514.00, 161675.70, 20.00, 3071838.30,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 646702.80, 646702.80, 3233514.00, 3136508.58, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 4233.66, 349.83, 4583.49, 11917074.00, 909558.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 2735.60, 226.04, 2961.64, 9477248.00, 723328.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff', 'c6c30c32-1190-4f28-9034-93956467475c', 2475.06, 204.52, 2679.58, 7770782.00, 593108.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff', '15393385-1d43-4ce7-93be-3cde99361431', 508.04, 41.98, 550.02, 13200480.00, 1007520.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-09-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-09-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-09-05 11:00:00+03' WHERE id='ce14bbef-9109-4cf0-bafe-6b8cabb9c7ff';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('fa257268-003b-4c90-b084-01fc93ef20f6', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 12, DATE '2026-09-01', DATE '2026-09-30', 'Draft',
    44486952.00, 42365584.00, 2121368.00, 106068.40, 20.00, 2015299.60,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 424273.60, 424273.60, 2121368.00, 2057726.96, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa257268-003b-4c90-b084-01fc93ef20f6', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 4583.49, 229.51, 4813.00, 12513800.00, 596726.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa257268-003b-4c90-b084-01fc93ef20f6', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 2961.64, 148.30, 3109.94, 9951808.00, 474560.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa257268-003b-4c90-b084-01fc93ef20f6', 'c6c30c32-1190-4f28-9034-93956467475c', 2679.58, 134.18, 2813.76, 8159904.00, 389122.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa257268-003b-4c90-b084-01fc93ef20f6', '15393385-1d43-4ce7-93be-3cde99361431', 550.02, 27.54, 577.56, 13861440.00, 660960.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-10-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-10-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-10-05 11:00:00+03' WHERE id='fa257268-003b-4c90-b084-01fc93ef20f6';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('b9703684-73c2-4602-82e9-61263fcc33b7', v_proj, 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 13, DATE '2026-10-01', DATE '2026-10-31', 'Draft',
    45248897.00, 44486952.00, 761945.00, 38097.25, 20.00, 723847.75,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 152389.00, 152389.00, 761945.00, 739086.65, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b9703684-73c2-4602-82e9-61263fcc33b7', '4d98d4fb-ae59-4fbc-91af-878bf23898bc', 4813.00, 82.45, 4895.45, 12728170.00, 214370.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b9703684-73c2-4602-82e9-61263fcc33b7', '77b9cf86-f52b-44bd-b955-b88ea6c79c29', 3109.94, 53.27, 3163.21, 10122272.00, 170464.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b9703684-73c2-4602-82e9-61263fcc33b7', 'c6c30c32-1190-4f28-9034-93956467475c', 2813.76, 48.19, 2861.95, 8299655.00, 139751.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b9703684-73c2-4602-82e9-61263fcc33b7', '15393385-1d43-4ce7-93be-3cde99361431', 577.56, 9.89, 587.45, 14098800.00, 237360.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-11-05 11:00:00+03' WHERE id='b9703684-73c2-4602-82e9-61263fcc33b7';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('1a506af9-2bf2-4bdc-9168-db20156c2f98', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 1, DATE '2026-03-01', DATE '2026-03-31', 'Draft',
    510580.00, 0.00, 510580.00, 25529.00, 20.00, 485051.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 102116.00, 102116.00, 510580.00, 495262.60, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1a506af9-2bf2-4bdc-9168-db20156c2f98', '066125af-cc64-4bad-a483-92b7f637434d', 0.00, 14.29, 14.29, 371540.00, 371540.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1a506af9-2bf2-4bdc-9168-db20156c2f98', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 0.00, 6.32, 6.32, 139040.00, 139040.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-04-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-04-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-04-05 11:00:00+03' WHERE id='1a506af9-2bf2-4bdc-9168-db20156c2f98';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('5bd5d7ad-6bca-4fa9-8cf4-af2f091d2d26', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 2, DATE '2026-04-01', DATE '2026-04-30', 'Draft',
    1856040.00, 510580.00, 1345460.00, 67273.00, 20.00, 1278187.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 269092.00, 269092.00, 1345460.00, 1305096.20, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('5bd5d7ad-6bca-4fa9-8cf4-af2f091d2d26', '066125af-cc64-4bad-a483-92b7f637434d', 14.29, 37.66, 51.95, 1350700.00, 979160.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('5bd5d7ad-6bca-4fa9-8cf4-af2f091d2d26', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 6.32, 16.65, 22.97, 505340.00, 366300.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-05-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-05-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-05-05 11:00:00+03' WHERE id='5bd5d7ad-6bca-4fa9-8cf4-af2f091d2d26';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('7c73f3ee-5f61-43a5-9cf5-f0e76cdc7fa1', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 3, DATE '2026-05-01', DATE '2026-05-31', 'Draft',
    3758680.00, 1856040.00, 1902640.00, 95132.00, 20.00, 1807508.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 380528.00, 380528.00, 1902640.00, 1845560.80, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('7c73f3ee-5f61-43a5-9cf5-f0e76cdc7fa1', '066125af-cc64-4bad-a483-92b7f637434d', 51.95, 53.26, 105.21, 2735460.00, 1384760.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('7c73f3ee-5f61-43a5-9cf5-f0e76cdc7fa1', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 22.97, 23.54, 46.51, 1023220.00, 517880.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-06-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-06-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-06-05 11:00:00+03' WHERE id='7c73f3ee-5f61-43a5-9cf5-f0e76cdc7fa1';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('025e38ea-c9ee-44e2-a182-0c6a24c31bf1', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 4, DATE '2026-06-01', DATE '2026-06-30', 'Draft',
    5939500.00, 3758680.00, 2180820.00, 109041.00, 20.00, 2071779.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 436164.00, 436164.00, 2180820.00, 2115395.40, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('025e38ea-c9ee-44e2-a182-0c6a24c31bf1', '066125af-cc64-4bad-a483-92b7f637434d', 105.21, 61.04, 166.25, 4322500.00, 1587040.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('025e38ea-c9ee-44e2-a182-0c6a24c31bf1', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 46.51, 26.99, 73.50, 1617000.00, 593780.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-07-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-07-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-07-05 11:00:00+03' WHERE id='025e38ea-c9ee-44e2-a182-0c6a24c31bf1';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('1d89f7ae-47cc-4edf-87ab-d8f9b2847cce', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 5, DATE '2026-07-01', DATE '2026-07-31', 'Draft',
    8120320.00, 5939500.00, 2180820.00, 109041.00, 20.00, 2071779.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 436164.00, 436164.00, 2180820.00, 2115395.40, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1d89f7ae-47cc-4edf-87ab-d8f9b2847cce', '066125af-cc64-4bad-a483-92b7f637434d', 166.25, 61.04, 227.29, 5909540.00, 1587040.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1d89f7ae-47cc-4edf-87ab-d8f9b2847cce', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 73.50, 26.99, 100.49, 2210780.00, 593780.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-08-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-08-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-08-05 11:00:00+03' WHERE id='1d89f7ae-47cc-4edf-87ab-d8f9b2847cce';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('b63ae7e9-f9a5-469f-927c-4c4df44657ff', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 6, DATE '2026-08-01', DATE '2026-08-31', 'Draft',
    10022960.00, 8120320.00, 1902640.00, 95132.00, 20.00, 1807508.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 380528.00, 380528.00, 1902640.00, 1845560.80, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b63ae7e9-f9a5-469f-927c-4c4df44657ff', '066125af-cc64-4bad-a483-92b7f637434d', 227.29, 53.26, 280.55, 7294300.00, 1384760.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('b63ae7e9-f9a5-469f-927c-4c4df44657ff', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 100.49, 23.54, 124.03, 2728660.00, 517880.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-09-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-09-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-09-05 11:00:00+03' WHERE id='b63ae7e9-f9a5-469f-927c-4c4df44657ff';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('d2a73068-92c6-435f-adb7-d58e62829e88', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 7, DATE '2026-09-01', DATE '2026-09-30', 'Draft',
    11368420.00, 10022960.00, 1345460.00, 67273.00, 20.00, 1278187.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 269092.00, 269092.00, 1345460.00, 1305096.20, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('d2a73068-92c6-435f-adb7-d58e62829e88', '066125af-cc64-4bad-a483-92b7f637434d', 280.55, 37.66, 318.21, 8273460.00, 979160.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('d2a73068-92c6-435f-adb7-d58e62829e88', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 124.03, 16.65, 140.68, 3094960.00, 366300.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-10-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-10-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-10-05 11:00:00+03' WHERE id='d2a73068-92c6-435f-adb7-d58e62829e88';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('554b21d5-33cb-4f3e-aef6-e261d95e3377', v_proj, 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 8, DATE '2026-10-01', DATE '2026-10-31', 'Draft',
    11879000.00, 11368420.00, 510580.00, 25529.00, 20.00, 485051.00,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 102116.00, 102116.00, 510580.00, 495262.60, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('554b21d5-33cb-4f3e-aef6-e261d95e3377', '066125af-cc64-4bad-a483-92b7f637434d', 318.21, 14.29, 332.50, 8645000.00, 371540.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('554b21d5-33cb-4f3e-aef6-e261d95e3377', 'f903f6fa-6a1e-4b78-91c4-c54d0a413fa5', 140.68, 6.32, 147.00, 3234000.00, 139040.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-11-05 11:00:00+03' WHERE id='554b21d5-33cb-4f3e-aef6-e261d95e3377';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('9737c7f6-ceba-4028-8572-94f76fe5a734', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 1, DATE '2026-06-01', DATE '2026-06-30', 'Draft',
    487552.00, 0.00, 487552.00, 24377.60, 20.00, 463174.40,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 97510.40, 97510.40, 487552.00, 472925.44, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('9737c7f6-ceba-4028-8572-94f76fe5a734', '0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', 0.00, 197.60, 197.60, 154128.00, 154128.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('9737c7f6-ceba-4028-8572-94f76fe5a734', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 0.00, 374.40, 374.40, 157248.00, 157248.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('9737c7f6-ceba-4028-8572-94f76fe5a734', '791aab51-1bab-4b47-8ccc-93f3bd38d7d6', 0.00, 24.96, 24.96, 46176.00, 46176.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('9737c7f6-ceba-4028-8572-94f76fe5a734', '7207d5b0-244d-4df4-9027-c6724f84479e', 0.00, 0.02, 0.02, 130000.00, 130000.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-07-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-07-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-07-05 11:00:00+03' WHERE id='9737c7f6-ceba-4028-8572-94f76fe5a734';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('0d85fbad-4a58-4915-a488-166335825e05', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 2, DATE '2026-07-01', DATE '2026-07-31', 'Draft',
    1665176.00, 487552.00, 1177624.00, 58881.20, 20.00, 1118742.80,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 235524.80, 235524.80, 1177624.00, 1142295.28, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('0d85fbad-4a58-4915-a488-166335825e05', '0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', 197.60, 471.20, 668.80, 521664.00, 367536.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('0d85fbad-4a58-4915-a488-166335825e05', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 374.40, 892.80, 1267.20, 532224.00, 374976.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('0d85fbad-4a58-4915-a488-166335825e05', '791aab51-1bab-4b47-8ccc-93f3bd38d7d6', 24.96, 59.52, 84.48, 156288.00, 110112.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('0d85fbad-4a58-4915-a488-166335825e05', '7207d5b0-244d-4df4-9027-c6724f84479e', 0.02, 0.05, 0.07, 455000.00, 325000.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-08-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-08-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-08-05 11:00:00+03' WHERE id='0d85fbad-4a58-4915-a488-166335825e05';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('a6299626-9d3d-4728-aae0-b918b5bf894d', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 3, DATE '2026-08-01', DATE '2026-08-31', 'Draft',
    3072824.00, 1665176.00, 1407648.00, 70382.40, 20.00, 1337265.60,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 281529.60, 281529.60, 1407648.00, 1365418.56, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a6299626-9d3d-4728-aae0-b918b5bf894d', '0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', 668.80, 562.40, 1231.20, 960336.00, 438672.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a6299626-9d3d-4728-aae0-b918b5bf894d', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 1267.20, 1065.60, 2332.80, 979776.00, 447552.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a6299626-9d3d-4728-aae0-b918b5bf894d', '791aab51-1bab-4b47-8ccc-93f3bd38d7d6', 84.48, 71.04, 155.52, 287712.00, 131424.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('a6299626-9d3d-4728-aae0-b918b5bf894d', '7207d5b0-244d-4df4-9027-c6724f84479e', 0.07, 0.06, 0.13, 845000.00, 390000.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-09-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-09-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-09-05 11:00:00+03' WHERE id='a6299626-9d3d-4728-aae0-b918b5bf894d';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('fa6db76d-2bce-4df3-a6e3-2c27aeff3210', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 4, DATE '2026-09-01', DATE '2026-09-30', 'Draft',
    4250448.00, 3072824.00, 1177624.00, 58881.20, 20.00, 1118742.80,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 235524.80, 235524.80, 1177624.00, 1142295.28, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa6db76d-2bce-4df3-a6e3-2c27aeff3210', '0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', 1231.20, 471.20, 1702.40, 1327872.00, 367536.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa6db76d-2bce-4df3-a6e3-2c27aeff3210', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 2332.80, 892.80, 3225.60, 1354752.00, 374976.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa6db76d-2bce-4df3-a6e3-2c27aeff3210', '791aab51-1bab-4b47-8ccc-93f3bd38d7d6', 155.52, 59.52, 215.04, 397824.00, 110112.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('fa6db76d-2bce-4df3-a6e3-2c27aeff3210', '7207d5b0-244d-4df4-9027-c6724f84479e', 0.13, 0.05, 0.18, 1170000.00, 325000.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-10-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-10-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-10-05 11:00:00+03' WHERE id='fa6db76d-2bce-4df3-a6e3-2c27aeff3210';
INSERT INTO progress_payments (id, project_id, subcontractor_id, period_no, period_start, period_end, status,
    gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable, submitted_at, site_approved_at, finalized_at,
    finalized_by, created_by, withholding_applied, vat_amount, vat_collected, payable_gross, actual_cost, is_final)
VALUES ('1552fd99-2c6e-4f40-94ef-bafb15a30195', v_proj, '995a0a86-2a04-4d9a-a986-312025cee542', 5, DATE '2026-10-01', DATE '2026-10-31', 'Draft',
    4738000.00, 4250448.00, 487552.00, 24377.60, 20.00, 463174.40,
    NULL, NULL, NULL,
    v_admin, v_admin, false, 97510.40, 97510.40, 487552.00, 472925.44, false);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1552fd99-2c6e-4f40-94ef-bafb15a30195', '0933b31c-4b7c-4818-89d2-2b7d6a5f6f0d', 1702.40, 197.60, 1900.00, 1482000.00, 154128.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1552fd99-2c6e-4f40-94ef-bafb15a30195', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 3225.60, 374.40, 3600.00, 1512000.00, 157248.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1552fd99-2c6e-4f40-94ef-bafb15a30195', '791aab51-1bab-4b47-8ccc-93f3bd38d7d6', 215.04, 24.96, 240.00, 444000.00, 46176.00);
INSERT INTO progress_payment_items (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
VALUES ('1552fd99-2c6e-4f40-94ef-bafb15a30195', '7207d5b0-244d-4df4-9027-c6724f84479e', 0.18, 0.02, 0.20, 1300000.00, 130000.00);
UPDATE progress_payments SET status='Finalized', submitted_at=TIMESTAMPTZ '2026-11-05 09:00:00+03', site_approved_at=TIMESTAMPTZ '2026-11-05 10:00:00+03', finalized_at=TIMESTAMPTZ '2026-11-05 11:00:00+03' WHERE id='1552fd99-2c6e-4f40-94ef-bafb15a30195';

  -- 6) İDARİ HAKEDİŞ (işveren hakedişleri; finansal ilerleme)
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 1, '2025-07 dönemi hakedişi', 1048998.35, 20.00,
        'FT-2026-0001', DATE '2025-08-30', v_admin, DATE '2025-07-31', 0.00, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 2, '2025-08 dönemi hakedişi', 2811319.76, 20.00,
        'FT-2026-0002', DATE '2025-09-30', v_admin, DATE '2025-08-31', 1048998.35, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 3, '2025-09 dönemi hakedişi', 4070117.89, 20.00,
        'FT-2026-0003', DATE '2025-10-30', v_admin, DATE '2025-09-30', 3860318.11, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 4, '2025-10 dönemi hakedişi', 5889647.37, 20.00,
        'FT-2026-0004', DATE '2025-11-30', v_admin, DATE '2025-10-31', 7930436.00, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 5, '2025-11 dönemi hakedişi', 8040135.19, 20.00,
        'FT-2026-0005', DATE '2025-12-30', v_admin, DATE '2025-11-30', 13820083.37, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 6, '2025-12 dönemi hakedişi', 9341887.70, 20.00,
        'FT-2026-0006', DATE '2026-01-30', v_admin, DATE '2025-12-31', 21860218.56, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 7, '2026-01 dönemi hakedişi', 9794740.25, 20.00,
        'FT-2026-0007', DATE '2026-03-02', v_admin, DATE '2026-01-31', 31202106.26, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 8, '2026-02 dönemi hakedişi', 9399320.66, 20.00,
        'FT-2026-0008', DATE '2026-03-30', v_admin, DATE '2026-02-28', 40996846.51, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 9, '2026-03 dönemi hakedişi', 8867431.07, 20.00,
        'FT-2026-0009', DATE '2026-04-30', v_admin, DATE '2026-03-31', 50396167.17, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 10, '2026-04 dönemi hakedişi', 9157408.76, 20.00,
        'FT-2026-0010', DATE '2026-05-30', v_admin, DATE '2026-04-30', 59263598.24, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 11, '2026-05 dönemi hakedişi', 9763172.17, 20.00,
        'FT-2026-0011', DATE '2026-06-30', v_admin, DATE '2026-05-31', 68421007.00, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 12, '2026-06 dönemi hakedişi', 10314669.05, 20.00,
        'FT-2026-0012', DATE '2026-07-30', v_admin, DATE '2026-06-30', 78184179.17, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 13, '2026-07 dönemi hakedişi', 10415573.05, 20.00,
        'FT-2026-0013', DATE '2026-08-30', v_admin, DATE '2026-07-31', 88498848.22, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 14, '2026-08 dönemi hakedişi', 9140054.14, 20.00,
        'FT-2026-0014', DATE '2026-09-30', v_admin, DATE '2026-08-31', 98914421.27, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 15, '2026-09 dönemi hakedişi', 6487137.41, 20.00,
        'FT-2026-0015', DATE '2026-10-30', v_admin, DATE '2026-09-30', 108054475.41, '[]'::jsonb);
INSERT INTO idari_hakedisler (project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi,
        created_by, hakedis_tarihi, onceki_hakedis_toplami, kesintiler)
    VALUES (v_proj, 16, '2026-10 dönemi hakedişi', 2458387.18, 20.00,
        'FT-2026-0016', NULL, v_admin, DATE '2026-10-31', 114541612.82, '[]'::jsonb);

  -- 7) İŞ PROGRAMI KİLOMETRE TAŞLARI (PV kaynağı: milestone yedeği)
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, 'Şantiye kurulumu ve mobilizasyon', DATE '2025-06-30', DATE '2025-07-05', 5.00, 'Completed', 1);
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, 'Temel tamamlanması', DATE '2026-01-31', DATE '2026-01-20', 15.00, 'Completed', 2);
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, '1. kat kaba inşaat tamamlanması', DATE '2026-05-31', DATE '2026-06-09', 20.00, 'Completed', 3);
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, 'Çelik çatı montajı', DATE '2026-09-30', NULL, 20.00, 'Delayed', 4);
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, 'Kaba inşaat tamamlanması', DATE '2026-12-31', NULL, 25.00, 'Planned', 5);
INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
VALUES (v_proj, 'MEP ön kabul ve test', DATE '2027-06-30', NULL, 15.00, 'Planned', 6);

  -- 8) PV PLANI (aylık planlanan % — kümülatif toplam = %100)
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-06-01', 0.2565);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-07-01', 0.7670);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-08-01', 1.2695);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-09-01', 1.7591);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-10-01', 2.2306);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-11-01', 2.6792);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2025-12-01', 3.1003);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-01-01', 3.4896);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-02-01', 3.8430);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-03-01', 4.1571);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-04-01', 4.4285);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-05-01', 4.6544);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-06-01', 4.8326);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-07-01', 4.9612);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-08-01', 5.0389);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-09-01', 5.0649);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-10-01', 5.0389);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-11-01', 4.9612);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2026-12-01', 4.8326);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-01-01', 4.6544);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-02-01', 4.4285);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-03-01', 4.1571);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-04-01', 3.8430);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-05-01', 3.4896);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-06-01', 3.1003);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-07-01', 2.6792);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-08-01', 2.2306);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-09-01', 1.7591);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-10-01', 1.2695);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-11-01', 0.7670);
INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES (v_proj, DATE '2027-12-01', 0.2565);

  -- 9) SATINALMA: PR + PO + TESLİMAT
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('cfddbd71-98e7-433f-966f-7bfb3c520c4c', v_proj, 'TLP-2025-001', v_admin, DATE '2026-02-20', 'Draft', NULL, NULL, NULL, NULL, '35719ad9-39fa-496d-8dce-53ae353d452a');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('cfddbd71-98e7-433f-966f-7bfb3c520c4c', 'Çelik profil malzeme tedariki', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-02-20 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-02-20 10:00:00+03', decision_note='Onaylandı' WHERE id='cfddbd71-98e7-433f-966f-7bfb3c520c4c';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('512c4e21-3f3e-4720-a362-85c49c9a8a71', v_proj, 'cfddbd71-98e7-433f-966f-7bfb3c520c4c', 'SIP-2025-001', 'Mavi Çelik Tic. Ltd.', 12000000.00, 'TRY', 'Ordered', DATE '2026-02-20', v_admin, '35719ad9-39fa-496d-8dce-53ae353d452a');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('512c4e21-3f3e-4720-a362-85c49c9a8a71', 'IRS-0001', TIMESTAMPTZ '2026-02-20 14:00:00+03', v_admin, 'Çelik profil malzeme tedariki teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='512c4e21-3f3e-4720-a362-85c49c9a8a71';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('951916e6-ea39-4bf0-8f81-c29e1ef70c36', v_proj, 'TLP-2025-002', v_admin, DATE '2026-05-15', 'Draft', NULL, NULL, NULL, NULL, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('951916e6-ea39-4bf0-8f81-c29e1ef70c36', 'Trafo ve ana pano', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-05-15 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-05-15 10:00:00+03', decision_note='Onaylandı' WHERE id='951916e6-ea39-4bf0-8f81-c29e1ef70c36';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('209bac81-d121-4ae1-8337-bcf1c429edd5', v_proj, '951916e6-ea39-4bf0-8f81-c29e1ef70c36', 'SIP-2025-002', 'Enerji Ekipman A.Ş.', 4800000.00, 'TRY', 'Ordered', DATE '2026-05-15', v_admin, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('209bac81-d121-4ae1-8337-bcf1c429edd5', 'IRS-0002', TIMESTAMPTZ '2026-05-15 14:00:00+03', v_admin, 'Trafo ve ana pano teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='209bac81-d121-4ae1-8337-bcf1c429edd5';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('92169d68-ecf0-4ee3-b42b-be51c8578a3f', v_proj, 'TLP-2025-003', v_admin, DATE '2026-07-02', 'Draft', NULL, NULL, NULL, NULL, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('92169d68-ecf0-4ee3-b42b-be51c8578a3f', 'Jeneratör ve UPS sistemi', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-07-02 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-07-02 10:00:00+03', decision_note='Onaylandı' WHERE id='92169d68-ecf0-4ee3-b42b-be51c8578a3f';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('3ac095b0-0cf8-4ece-8c66-ce42deef28d0', v_proj, '92169d68-ecf0-4ee3-b42b-be51c8578a3f', 'SIP-2025-003', 'Enerji Ekipman A.Ş.', 3200000.00, 'TRY', 'Ordered', DATE '2026-07-02', v_admin, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('3ac095b0-0cf8-4ece-8c66-ce42deef28d0', 'IRS-0003', TIMESTAMPTZ '2026-07-02 14:00:00+03', v_admin, 'Jeneratör ve UPS sistemi teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='3ac095b0-0cf8-4ece-8c66-ce42deef28d0';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('cf15e1f6-b8f3-4ab8-8c5d-65310387723a', v_proj, 'TLP-2025-004', v_admin, DATE '2026-06-18', 'Draft', NULL, NULL, NULL, NULL, 'a45ec96a-a5ac-4e41-8a32-04a4a5284f93');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('cf15e1f6-b8f3-4ab8-8c5d-65310387723a', 'Yangın söndürme tesisatı malzemesi', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-06-18 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-06-18 10:00:00+03', decision_note='Onaylandı' WHERE id='cf15e1f6-b8f3-4ab8-8c5d-65310387723a';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('16c075db-6f59-4609-9282-ea37e8196af4', v_proj, 'cf15e1f6-b8f3-4ab8-8c5d-65310387723a', 'SIP-2025-004', 'Yangın Sistemleri Ltd.', 2600000.00, 'TRY', 'Ordered', DATE '2026-06-18', v_admin, 'a45ec96a-a5ac-4e41-8a32-04a4a5284f93');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('16c075db-6f59-4609-9282-ea37e8196af4', 'IRS-0004', TIMESTAMPTZ '2026-06-18 14:00:00+03', v_admin, 'Yangın söndürme tesisatı malzemesi teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='16c075db-6f59-4609-9282-ea37e8196af4';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('f2bd6bf1-59ea-4835-b52a-1b38e5641130', v_proj, 'TLP-2025-005', v_admin, DATE '2025-08-05', 'Draft', NULL, NULL, NULL, NULL, 'b68ad254-f8a7-46b8-b77d-f432c2622061');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('f2bd6bf1-59ea-4835-b52a-1b38e5641130', 'Şantiye konteyneri ve altyapı', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2025-08-05 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2025-08-05 10:00:00+03', decision_note='Onaylandı' WHERE id='f2bd6bf1-59ea-4835-b52a-1b38e5641130';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('0a7351ec-dd97-4c09-bf8a-8a0f6385b3b8', v_proj, 'f2bd6bf1-59ea-4835-b52a-1b38e5641130', 'SIP-2025-005', 'Saha Lojistik Ltd.', 2400000.00, 'TRY', 'Ordered', DATE '2025-08-05', v_admin, 'b68ad254-f8a7-46b8-b77d-f432c2622061');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('0a7351ec-dd97-4c09-bf8a-8a0f6385b3b8', 'IRS-0005', TIMESTAMPTZ '2025-08-05 14:00:00+03', v_admin, 'Şantiye konteyneri ve altyapı teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='0a7351ec-dd97-4c09-bf8a-8a0f6385b3b8';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('6bc6be50-9e3a-4f1b-bb96-9514082244eb', v_proj, 'TLP-2025-006', v_admin, DATE '2026-09-12', 'Draft', NULL, NULL, NULL, NULL, '67b589fb-7045-47be-9d0a-379557334805');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('6bc6be50-9e3a-4f1b-bb96-9514082244eb', 'Cephe panel kaplama malzemesi', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-09-12 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-09-12 10:00:00+03', decision_note='Onaylandı' WHERE id='6bc6be50-9e3a-4f1b-bb96-9514082244eb';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('729ee746-30e5-4c55-b7ec-2d9677176986', v_proj, '6bc6be50-9e3a-4f1b-bb96-9514082244eb', 'SIP-2025-006', 'Yapı Kaplama A.Ş.', 6000000.00, 'TRY', 'Ordered', DATE '2026-09-12', v_admin, '67b589fb-7045-47be-9d0a-379557334805');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('729ee746-30e5-4c55-b7ec-2d9677176986', 'IRS-0006', TIMESTAMPTZ '2026-09-12 14:00:00+03', v_admin, 'Cephe panel kaplama malzemesi teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='729ee746-30e5-4c55-b7ec-2d9677176986';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('38974242-8946-468b-bbf9-b02ac6b1deef', v_proj, 'TLP-2025-007', v_admin, DATE '2026-10-05', 'Draft', NULL, NULL, NULL, NULL, 'a45ec96a-a5ac-4e41-8a32-04a4a5284f93');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('38974242-8946-468b-bbf9-b02ac6b1deef', 'Hidrolik asansör', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-10-05 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-10-05 10:00:00+03', decision_note='Onaylandı' WHERE id='38974242-8946-468b-bbf9-b02ac6b1deef';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('571f8793-dadb-4bc0-aedd-4a9b328483a9', v_proj, '38974242-8946-468b-bbf9-b02ac6b1deef', 'SIP-2025-007', 'Asansör Dünyası Ltd.', 4000000.00, 'TRY', 'Ordered', DATE '2026-10-05', v_admin, 'a45ec96a-a5ac-4e41-8a32-04a4a5284f93');
INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, note) VALUES ('571f8793-dadb-4bc0-aedd-4a9b328483a9', 'IRS-0007', TIMESTAMPTZ '2026-10-05 14:00:00+03', v_admin, 'Hidrolik asansör teslim alındı');
UPDATE purchase_orders SET status='Delivered' WHERE id='571f8793-dadb-4bc0-aedd-4a9b328483a9';
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('65ee88e6-2129-45fd-a115-1bf811688925', v_proj, 'TLP-2026-008', v_admin, DATE '2026-12-15', 'Draft', NULL, NULL, NULL, NULL, '67b589fb-7045-47be-9d0a-379557334805');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('65ee88e6-2129-45fd-a115-1bf811688925', 'Cephe alüminyum doğrama', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-10-20 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-10-21 10:00:00+03', decision_note='Onaylandı' WHERE id='65ee88e6-2129-45fd-a115-1bf811688925';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('0b9f66c3-cc96-440a-8d4c-93f971b0f9ed', v_proj, '65ee88e6-2129-45fd-a115-1bf811688925', 'SIP-2026-008', 'Alüm Doğrama Sanayi', 4500000.00, 'TRY', 'Ordered', DATE '2026-12-15', v_admin, '67b589fb-7045-47be-9d0a-379557334805');
INSERT INTO purchase_requests (id, project_id, pr_no, requested_by, needed_by_date, status, submitted_at, decided_by, decided_at, decision_note, cost_code_id)
VALUES ('b7dac08a-57ee-4f8f-bbc9-c1445c1486ed', v_proj, 'TLP-2026-009', v_admin, DATE '2026-11-30', 'Draft', NULL, NULL, NULL, NULL, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');
INSERT INTO purchase_request_items (pr_id, material_name, qty, unit) VALUES ('b7dac08a-57ee-4f8f-bbc9-c1445c1486ed', 'Güvenlik ve CCTV sistemi', 1, 'ad');
UPDATE purchase_requests SET status='Approved', submitted_at=TIMESTAMPTZ '2026-10-20 09:00:00+03', decided_by=v_admin, decided_at=TIMESTAMPTZ '2026-10-21 10:00:00+03', decision_note='Onaylandı' WHERE id='b7dac08a-57ee-4f8f-bbc9-c1445c1486ed';
INSERT INTO purchase_orders (id, project_id, pr_id, po_no, supplier_name, amount, currency, status, expected_date, created_by, cost_code_id)
VALUES ('573b7088-730e-4e32-b996-93ab876c56d8', v_proj, 'b7dac08a-57ee-4f8f-bbc9-c1445c1486ed', 'SIP-2026-009', 'Güven Güvenlik Sist.', 1500000.00, 'TRY', 'Ordered', DATE '2026-11-30', v_admin, 'f5316db0-2da3-41e4-a5f9-f8e94ef751fb');

  -- 10) KARARLAR (karar günlüğü)
INSERT INTO project_decisions (project_id, decision_text, decided_on, created_by) VALUES (v_proj, 'Çelik çatı montaj sırası değiştirildi: önce kuzey açıklık tamamlanacak.', DATE '2026-02-12', v_admin);
INSERT INTO project_decisions (project_id, decision_text, decided_on, created_by) VALUES (v_proj, 'Trafo odası konumu mimari projeye uygun hale getirildi (revizyon R2).', DATE '2026-06-20', v_admin);
INSERT INTO project_decisions (project_id, decision_text, decided_on, created_by) VALUES (v_proj, 'Cephe panel tedariki tek sözleşmeyle yapılacak; taşeron iş kapsamı dışında.', DATE '2026-09-25', v_admin);
  -- 11) İŞ PROGRAMI (WBS kalemleri; baseline + gerçekleşen)
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '1', 'Şantiye kurulumu ve mobilizasyon', 10, false,
    DATE '2025-06-01', DATE '2025-07-05', DATE '2025-06-01', DATE '2025-07-05',
    'manual', 100.00, 2.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '2', 'Hafriyat ve altyapı', 20, false,
    DATE '2025-07-01', DATE '2026-03-31', DATE '2025-07-01', DATE '2026-03-20',
    'manual', 100.00, 10.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '3', 'Kaba inşaat', 30, false,
    DATE '2025-10-01', DATE '2026-12-31', DATE '2025-10-01', NULL,
    'manual', 72.00, 40.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '3.1', 'Temel betonu', 31, false,
    DATE '2025-10-01', DATE '2026-01-31', DATE '2025-10-01', DATE '2026-01-20',
    'manual', 100.00, 10.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '3.2', '1. kat kaba inşaat', 32, false,
    DATE '2026-02-01', DATE '2026-05-31', DATE '2026-02-01', DATE '2026-06-09',
    'manual', 100.00, 12.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '3.3', '2. kat ve üst kat kaba inşaat', 33, false,
    DATE '2026-06-01', DATE '2026-12-31', DATE '2026-06-10', NULL,
    'manual', 45.00, 18.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '4', 'Çelik konstrüksiyon', 40, false,
    DATE '2026-03-01', DATE '2026-09-30', DATE '2026-03-15', NULL,
    'manual', 40.00, 20.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '4.1', 'Kolon-kiriş montajı', 41, false,
    DATE '2026-03-01', DATE '2026-08-31', DATE '2026-03-15', NULL,
    'manual', 70.00, 12.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '4.2', 'Çatı makas ve gerdirme', 42, false,
    DATE '2026-09-01', DATE '2026-09-30', NULL, NULL,
    'manual', 0.00, 8.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '5', 'Mekanik ve elektrik tesisat', 50, false,
    DATE '2026-06-01', DATE '2027-06-30', DATE '2026-06-15', NULL,
    'manual', 15.00, 25.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '6', 'Cephe ve dış kaplama', 60, false,
    DATE '2027-01-01', DATE '2027-05-31', NULL, NULL,
    'manual', 0.00, 15.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '7', 'Peyzaj ve dış saha', 70, false,
    DATE '2027-05-01', DATE '2027-06-30', NULL, NULL,
    'manual', 0.00, 5.00);
INSERT INTO schedule_items (project_id, wbs_code, name, sort_order, is_milestone, baseline_start, baseline_finish,
    actual_start, actual_finish, progress_source, manual_progress, weight)
VALUES (v_proj, '8', 'Geçici kabul', 80, true,
    DATE '2027-07-01', DATE '2027-07-01', NULL, NULL,
    'manual', 0.00, 0.00);
  -- 12) GÜNLÜK SAHA RAPORLARI (son 9 iş günü; taslak -> gönder)
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', v_proj, DATE '2026-10-27', 1, '{"condition": "Güneşli"}'::jsonb, 12, 19,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 49);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 17);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 11);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'Kule vinç', 2, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'Beton pompası', 1, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', 'c6c30c32-1190-4f28-9034-93956467475c', 'B Blok 2. kat', 'Döşeme betonu dökümü', 180, 'm3', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 871, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9a64989a-b579-4f04-b677-6f03a6574386', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 8, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-10-27 18:00:00+03', submitted_by=v_admin WHERE id='9a64989a-b579-4f04-b677-6f03a6574386';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', v_proj, DATE '2026-10-28', 1, '{"condition": "Parçalı bulutlu"}'::jsonb, 7, 17,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 52);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 21);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 15);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', 'Kule vinç', 2, 10.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', 'Beton pompası', 1, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', '15393385-1d43-4ce7-93be-3cde99361431', 'B Blok 2. kat', 'Nervürlü donatı montajı', 8, 'ton', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 809, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('36e7993e-f283-42e8-8e78-9e9794dd56e1', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 5, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-10-28 18:00:00+03', submitted_by=v_admin WHERE id='36e7993e-f283-42e8-8e78-9e9794dd56e1';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', v_proj, DATE '2026-10-29', 1, '{"condition": "Parçalı bulutlu"}'::jsonb, 12, 18,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 52);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 22);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 13);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'Kule vinç', 2, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'Beton pompası', 1, 6.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', 'c6c30c32-1190-4f28-9034-93956467475c', 'B Blok 2. kat', 'Döşeme betonu dökümü', 135, 'm3', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 389, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('0b7e446b-93ef-4a23-9f5b-2956a38a6004', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 8, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-10-29 18:00:00+03', submitted_by=v_admin WHERE id='0b7e446b-93ef-4a23-9f5b-2956a38a6004';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', v_proj, DATE '2026-10-30', 1, '{"condition": "Yağışlı"}'::jsonb, 12, 19,
    'Hava koşulları nedeniyle kısmi çalışma yapıldı.',
    'Draft', v_admin, '[{"reason_code": "Hava", "hours": 3.5, "note": "Yağış nedeniyle beton dökümü ertelendi"}]'::jsonb, true);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 52);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 19);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 11);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', 'Kule vinç', 2, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', 'Beton pompası', 1, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', '15393385-1d43-4ce7-93be-3cde99361431', 'B Blok 2. kat', 'Nervürlü donatı montajı', 9, 'ton', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 504, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('6f0ff143-2b2f-4782-a5fa-efe4111e4100', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 9, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-10-30 18:00:00+03', submitted_by=v_admin WHERE id='6f0ff143-2b2f-4782-a5fa-efe4111e4100';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', v_proj, DATE '2026-11-02', 1, '{"condition": "Güneşli"}'::jsonb, 10, 22,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 53);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 21);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 14);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'Kule vinç', 2, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'Beton pompası', 1, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'Hidrolik ekskavatör', 1, 8.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', 'c6c30c32-1190-4f28-9034-93956467475c', 'B Blok 2. kat', 'Döşeme betonu dökümü', 214, 'm3', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 517, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('9b46f2b8-622a-47d3-88a1-a929d35b570e', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 9, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-11-02 18:00:00+03', submitted_by=v_admin WHERE id='9b46f2b8-622a-47d3-88a1-a929d35b570e';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', v_proj, DATE '2026-11-03', 1, '{"condition": "Bulutlu"}'::jsonb, 8, 16,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 45);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 20);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 13);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', 'Kule vinç', 2, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', 'Beton pompası', 1, 7.50, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', '15393385-1d43-4ce7-93be-3cde99361431', 'B Blok 2. kat', 'Nervürlü donatı montajı', 12, 'ton', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 755, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 9, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-11-03 18:00:00+03', submitted_by=v_admin WHERE id='07df62e6-1d1a-4d1f-9b1c-0c26b80b7b06';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', v_proj, DATE '2026-11-04', 1, '{"condition": "Güneşli"}'::jsonb, 12, 16,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 49);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 20);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 10);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'Kule vinç', 2, 9.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'Beton pompası', 1, 7.50, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'Hidrolik ekskavatör', 1, 8.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', 'c6c30c32-1190-4f28-9034-93956467475c', 'B Blok 2. kat', 'Döşeme betonu dökümü', 187, 'm3', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 553, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('3437a2b2-4259-495d-8a20-f0fd893bf475', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 5, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-11-04 18:00:00+03', submitted_by=v_admin WHERE id='3437a2b2-4259-495d-8a20-f0fd893bf475';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', v_proj, DATE '2026-11-05', 1, '{"condition": "Rüzgarlı"}'::jsonb, 10, 22,
    'Hava koşulları nedeniyle kısmi çalışma yapıldı.',
    'Draft', v_admin, '[{"reason_code": "Hava", "hours": 2.0, "note": "Rüzgâr 50 km/s üzeri, vinç çalışması durduruldu"}]'::jsonb, true);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 50);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 21);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 11);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', 'Kule vinç', 2, 9.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', 'Beton pompası', 1, 0.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', 'Hidrolik ekskavatör', 1, 0.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', '15393385-1d43-4ce7-93be-3cde99361431', 'B Blok 2. kat', 'Nervürlü donatı montajı', 8, 'ton', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 807, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('5d3ad04e-fba5-4f49-ab54-be66e756cc57', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 9, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-11-05 18:00:00+03', submitted_by=v_admin WHERE id='5d3ad04e-fba5-4f49-ab54-be66e756cc57';
INSERT INTO daily_reports (id, project_id, report_date, revision_no, weather, temperature_min, temperature_max,
    notes, status, author_id, delay_causes, issue_flagged)
VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', v_proj, DATE '2026-11-06', 1, '{"condition": "Güneşli"}'::jsonb, 6, 20,
    'Kaba inşaat ve çelik montajı planlandığı gibi ilerledi.',
    'Draft', v_admin, '[]'::jsonb, false);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'ccc92ec6-fe50-4b95-8635-0c1aff517b46', 'Betonarme', 56);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'dfbbe050-abcf-40ed-9d53-2bf03b4ee5e6', 'Çelik Konstrüksiyon', 19);
INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', '995a0a86-2a04-4d9a-a986-312025cee542', 'Mekanik-Elektrik', 10);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'Kule vinç', 2, 9.00, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'Beton pompası', 1, 7.50, NULL);
INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason) VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'Hidrolik ekskavatör', 1, 8.00, NULL);
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', 'c6c30c32-1190-4f28-9034-93956467475c', 'B Blok 2. kat', 'Döşeme betonu dökümü', 208, 'm3', 'Kaba inşaat');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', '8430b3e2-233f-4a9c-b271-a7337691bf2f', 'A Blok tavan arası', 'Elektrik kablo kanalı çekimi', 800, 'm', 'Elektrik');
INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit, discipline)
VALUES ('ba575aca-65e1-4637-a92b-59ecad7e52f6', '066125af-cc64-4bad-a483-92b7f637434d', 'Çelik kolon montajı - A aks', 'Kolon-kiriş montajı', 4, 'ton', 'Çelik');
UPDATE daily_reports SET status='Submitted', submitted_at=TIMESTAMPTZ '2026-11-06 18:00:00+03', submitted_by=v_admin WHERE id='ba575aca-65e1-4637-a92b-59ecad7e52f6';
END $$;
