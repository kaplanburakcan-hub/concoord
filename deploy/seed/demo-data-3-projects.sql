-- ============================================================================
-- İPKS — 3 DEMO PROJESİ (DEMO-02 / DEMO-03 / DEMO-04)
-- ============================================================================
-- İçerik (her proje için):
--   · Taşeronlar (4 adet) + sözleşmeleri + birim fiyat pozları
--   · Kesinleşmiş hakedişler (4-6 dönem) + onay bekleyen + taslak
--   · Avans/teminat/İSG ceza/yemek kesintileri
--   · Milestone'lar (6 adet)
--   · PV planı (EVM "manual" kaynağı)
--   · İSG bulgular (6-9 adet, farklı severity/durum)
--   · Malzeme onayları (6-8 adet)
--   · Görevler/Kanban (10-12 adet)
--   · Günlük saha raporları (5 adet: personel, ekipman, imalat)
--   · Satınalma talepleri (PR) + kalemleri — tedarikçi verisi
--   · Satın alma siparişleri (PO) + teslimatlar
--
-- GÜVENLİ: yalnızca kendi eklediği DEMO-0x projelerini oluşturur; idempotent.
-- Çalıştırma:
--   docker compose -f deploy/docker-compose.yml --env-file .env exec -T postgres \
--     psql -U ipks -d ipks < deploy/seed/demo-data-3-projects.sql
-- ============================================================================

-- ============================================================================
-- PROJE 1: DEMO-02 — Karayolu Yenileme ve Köprü Yapımı
-- ============================================================================
DO $$
DECLARE
  v_admin   uuid;
  v_role_pm uuid;
  v_proj    uuid;
  -- taşeronlar
  s_toprak uuid; s_beton uuid; s_kaplama uuid; s_kopru uuid;
  -- sözleşmeler
  c_toprak uuid; c_beton uuid; c_kaplama uuid; c_kopru uuid;
  -- pozlar
  w_hafriyat uuid; w_dolgu uuid; w_menfez uuid; w_asfalt uuid; w_kopru_beton uuid;
  -- hakediş
  pp        uuid;
  v_month   date;
  i         int; j int;
  gross_cum_arr numeric[] := ARRAY[12000000, 25000000, 39000000, 52000000];
  prev_cum  numeric := 0;
  this_amt  numeric;
  ded_adv   numeric; ded_ret  numeric; ded_wht  numeric;
  ded_ohs   numeric; ded_meal numeric; ded_util numeric;
  v_vat     numeric; v_vat_wh numeric; v_vat_coll numeric;
  v_payable numeric; v_ded_total numeric; v_cost_red numeric; v_actual numeric;
  net_amt   numeric;
  -- satınalma
  pr1 uuid; pr2 uuid; po1 uuid; po2 uuid;
  -- saha raporları
  dr uuid;
BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  IF v_admin IS NULL THEN
    RAISE EXCEPTION 'Kullanıcı bulunamadı — önce "make seed" çalıştırın.';
  END IF;
  SELECT id INTO v_role_pm FROM roles WHERE name = 'ProjectManager' LIMIT 1;
  IF v_role_pm IS NULL THEN
    SELECT id INTO v_role_pm FROM roles ORDER BY created_at LIMIT 1;
  END IF;

  -- Temizlik
  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-02';
  IF v_proj IS NOT NULL THEN
    UPDATE projects SET code = 'DEMO-02-ARSIV-' || to_char(now(),'YYYYMMDDHH24MISS'),
      name = name || ' (arşiv)', status = 'Archived', deleted_at = now()
    WHERE id = v_proj;
  END IF;

  -- ─── 1) PROJE ───────────────────────────────────────────────────────────
  INSERT INTO projects (code, name, location, client_name, budget_total,
                        currency, start_date, end_date, status)
  VALUES ('DEMO-02', 'Polatlı–Sivrihisar Otoyol ve Köprü Yapımı',
          'Ankara / Polatlı', 'Karayolları Genel Müdürlüğü',
          145000000, 'TRY', DATE '2025-01-01', DATE '2026-12-31', 'Active')
  RETURNING id INTO v_proj;

  INSERT INTO project_members (project_id, user_id, role_id)
  VALUES (v_proj, v_admin, v_role_pm);

  -- ─── 2) TAŞERONLAR ──────────────────────────────────────────────────────
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Güçlü Toprak ve Hafriyat A.Ş.','5001112233','Hasan Çelik','0537 100 1100','proje@guclutoprak.com','Toprak İşleri')
  RETURNING id INTO s_toprak;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Karadeniz Beton Yapı Ltd.','5002223344','Fatma Arslan','0532 200 2200','info@karadenizbeton.com','Betonarme')
  RETURNING id INTO s_beton;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Asfalt-Tek Yol Kaplama A.Ş.','5003334455','Kemal Şahin','0533 300 3300','proje@asfaltek.com','Yol Kaplama')
  RETURNING id INTO s_kaplama;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Köprü-İnş Mühendislik A.Ş.','5004445566','Sibel Güneş','0534 400 4400','info@kopruins.com','Köprü / Viyadük')
  RETURNING id INTO s_kopru;

  -- ─── 3) SÖZLEŞMELER ─────────────────────────────────────────────────────
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
    advance_amount, retention_pct, advance_rate_pct, sign_date, start_date, end_date,
    is_multi_year, withholding_pct)
  VALUES (v_proj,s_toprak,'SZL-2025-K01','Sub',55000000,8250000,5.00,20.00,
    DATE '2024-12-15', DATE '2025-01-01', DATE '2026-06-30', true, 5.00)
  RETURNING id INTO c_toprak;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
    advance_amount, retention_pct, advance_rate_pct, sign_date, start_date, end_date,
    is_multi_year, withholding_pct)
  VALUES (v_proj,s_beton,'SZL-2025-K02','Sub',38000000,5700000,5.00,15.00,
    DATE '2025-01-10', DATE '2025-02-01', DATE '2026-09-30', true, 5.00)
  RETURNING id INTO c_beton;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
    advance_amount, retention_pct, advance_rate_pct, sign_date)
  VALUES (v_proj,s_kaplama,'SZL-2025-K03','Sub',32000000,4800000,3.00,15.00, DATE '2025-04-01')
  RETURNING id INTO c_kaplama;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
    advance_amount, retention_pct, advance_rate_pct, sign_date)
  VALUES (v_proj,s_kopru,'SZL-2025-K04','Sub',20000000,3000000,5.00,20.00, DATE '2025-03-01')
  RETURNING id INTO c_kopru;

  -- ─── 4) BİRİM FİYAT POZLARI ─────────────────────────────────────────────
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_toprak,c_toprak,'Y.16.001','Kazı ve hafriyat taşınması','m3',380000,95.00)
  RETURNING id INTO w_hafriyat;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_toprak,c_toprak,'Y.16.080','Granüler dolgu malzemesi serimi','m3',290000,125.00)
  RETURNING id INTO w_dolgu;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_beton,c_beton,'Y.17.210','Betonarme menfez yapımı (B30)','m3',4200,2800.00)
  RETURNING id INTO w_menfez;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaplama,c_kaplama,'Y.40.500','Bitümlü sıcak karışım üst tabaka (BBM)','ton',68000,1950.00)
  RETURNING id INTO w_asfalt;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kopru,c_kopru,'Y.18.700','Köprü tabliye betonarme (C40/50)','m3',5800,4200.00)
  RETURNING id INTO w_kopru_beton;

  -- ─── 5) HAKEDİŞLER (4 KEŞİNLEŞMİŞ) ─────────────────────────────────────
  FOR i IN 1..4 LOOP
    v_month  := (DATE '2025-01-01' + ((i-1)||' month')::interval)::date;
    this_amt := gross_cum_arr[i] - prev_cum;
    ded_adv  := round(this_amt * 0.20, 2);
    ded_ret  := round(this_amt * 0.05, 2);
    ded_wht  := round(this_amt * 0.05, 2);
    ded_ohs  := CASE WHEN i IN (2,4) THEN 55000 ELSE 0 END;
    ded_meal := round(this_amt * 0.014, 2);
    ded_util := round(this_amt * 0.009, 2);
    v_vat        := round(this_amt * 0.20, 2);
    v_vat_wh     := round(v_vat * 0.40, 2);
    v_vat_coll   := v_vat - v_vat_wh;
    v_payable    := this_amt + v_vat_coll;
    v_ded_total  := ded_adv + ded_ret + ded_wht + ded_ohs + ded_meal + ded_util;
    net_amt      := v_payable - v_ded_total;
    v_cost_red   := ded_ohs + round(ded_meal/1.10,2) + round(ded_util/1.20,2);
    v_actual     := this_amt - v_cost_red;

    INSERT INTO progress_payments (
      project_id, subcontractor_id, period_no, period_start, period_end, status,
      gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable,
      vat_amount, vat_withheld, vat_collected, payable_gross, actual_cost,
      vat_withholding_ratio, withholding_applied, current_step_no,
      submitted_at, site_approved_at, finalized_at, finalized_by, created_by, created_at)
    VALUES (v_proj, s_toprak, i,
      date_trunc('month',v_month)::date,
      (date_trunc('month',v_month)+interval '1 month - 1 day')::date,
      'SiteApproved',
      gross_cum_arr[i], prev_cum, this_amt, v_ded_total, 20.00, net_amt,
      v_vat, v_vat_wh, v_vat_coll, v_payable, v_actual,
      0.40, true, 9,
      (v_month+interval '25 day'),(v_month+interval '27 day'),
      (v_month+interval '28 day'), v_admin, v_admin,(v_month+interval '20 day'))
    RETURNING id INTO pp;

    INSERT INTO payment_deductions
      (progress_payment_id,type,description,rate_pct,amount,nature,reduces_cost,
       group_code,catalog_code,vat_pct,net_amount)
    VALUES
      (pp,'AdvanceOffset','Avans mahsubu',20.00,ded_adv,'Offset',false,'Advance','ADV_CONTRACT',0,ded_adv),
      (pp,'Retention','Teminat kesintisi',5.00,ded_ret,'Temporary',false,'Retention','RET_PERFORMANCE',0,ded_ret),
      (pp,'Withholding','Stopaj (yıllara sari)',5.00,ded_wht,'Permanent',false,'Tax','WHT_INCOME',0,ded_wht),
      (pp,'Other','Öğle yemeği bedeli',NULL,ded_meal,'Permanent',true,'GoodsService','GS_LUNCH',10,round(ded_meal/1.10,2)),
      (pp,'Other','Elektrik ve su bedeli',NULL,ded_util,'Permanent',true,'GoodsService','GS_ELECTRICITY',20,round(ded_util/1.20,2));

    IF ded_ohs > 0 THEN
      INSERT INTO payment_deductions
        (progress_payment_id,type,source_entity,description,amount,nature,reduces_cost,
         group_code,catalog_code,vat_pct,net_amount)
      VALUES (pp,'OHSPenalty','ohs_penalties','İSG ceza tutanağı kesintisi',ded_ohs,'Permanent',true,'Penalty','PEN_OHS',0,ded_ohs);
    END IF;

    INSERT INTO progress_payment_items
      (progress_payment_id,work_item_id,prev_cum_qty,this_period_qty,cum_qty,cum_amount,this_amount)
    VALUES
      (pp,w_hafriyat,round((prev_cum/95.0)*0.40,3),round((this_amt/95.0)*0.40,3),
        round((gross_cum_arr[i]/95.0)*0.40,3),round(gross_cum_arr[i]*0.40,2),round(this_amt*0.40,2)),
      (pp,w_dolgu,round((prev_cum/125.0)*0.35,3),round((this_amt/125.0)*0.35,3),
        round((gross_cum_arr[i]/125.0)*0.35,3),round(gross_cum_arr[i]*0.35,2),round(this_amt*0.35,2)),
      (pp,w_menfez,round((prev_cum/2800.0)*0.25,3),round((this_amt/2800.0)*0.25,3),
        round((gross_cum_arr[i]/2800.0)*0.25,3),round(gross_cum_arr[i]*0.25,2),round(this_amt*0.25,2));

    FOR j IN 1..9 LOOP
      INSERT INTO payment_approvals (progress_payment_id,step_no,step_code,decision,actor_id,created_at)
      SELECT pp, s.step_no, s.code, 'Approved', v_admin, (v_month+interval '27 day')+(j||' hour')::interval
      FROM payment_approval_steps s WHERE s.project_id IS NULL AND s.step_no = j;
    END LOOP;

    UPDATE progress_payments SET status = 'Finalized' WHERE id = pp;
    prev_cum := gross_cum_arr[i];
  END LOOP;

  -- 5. dönem — SiteApproved (onay bekliyor)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,
    vat_amount,vat_withheld,vat_collected,payable_gross,actual_cost,
    vat_withholding_ratio,withholding_applied,current_step_no,
    submitted_at,site_approved_at,created_by,created_at)
  VALUES (v_proj,s_toprak,5,DATE '2025-05-01',DATE '2025-05-31','SiteApproved',
    62500000,52000000,10500000,2625000,20.00,8400000,
    2100000,840000,1260000,11760000,9750000,0.40,true,5,
    now()-interval '10 day',now()-interval '8 day',v_admin,now()-interval '12 day');

  -- 6. dönem — Draft (kaplama taşeronuna)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,created_by,created_at)
  VALUES (v_proj,s_kaplama,1,DATE '2025-06-01',DATE '2025-06-30','Draft',
    8200000,0,8200000,2050000,20.00,6150000,v_admin,now()-interval '3 day');

  -- ─── 6) MİLESTONE'LAR ───────────────────────────────────────────────────
  INSERT INTO milestones (project_id,name,planned_date,actual_date,weight_pct,status,sort_order) VALUES
    (v_proj,'Şantiye kurulumu ve mobilizasyon',DATE '2025-02-01',DATE '2025-01-28',3.00,'Completed',1),
    (v_proj,'Hafriyat ve dolgu işleri tamamlanması',DATE '2025-07-31',DATE '2025-08-10',22.00,'Completed',2),
    (v_proj,'Menfez ve alt yapı betonarme',DATE '2025-10-31',NULL,18.00,'InProgress',3),
    (v_proj,'Köprü tabliye betonarme ve kutu kiriş',DATE '2026-03-31',NULL,25.00,'Planned',4),
    (v_proj,'Asfalt kaplama ve yol çizgisi',DATE '2026-08-31',NULL,20.00,'Planned',5),
    (v_proj,'Tamamlama, çevre düzenleme ve geçici kabul',DATE '2026-12-15',NULL,12.00,'Planned',6);

  -- ─── 7) PV PLANI (24 ay) ────────────────────────────────────────────────
  INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES
    (v_proj,DATE '2025-01-01',2.50),(v_proj,DATE '2025-02-01',3.50),
    (v_proj,DATE '2025-03-01',4.50),(v_proj,DATE '2025-04-01',5.50),
    (v_proj,DATE '2025-05-01',5.50),(v_proj,DATE '2025-06-01',5.50),
    (v_proj,DATE '2025-07-01',6.00),(v_proj,DATE '2025-08-01',6.00),
    (v_proj,DATE '2025-09-01',5.50),(v_proj,DATE '2025-10-01',5.00),
    (v_proj,DATE '2025-11-01',5.00),(v_proj,DATE '2025-12-01',4.50),
    (v_proj,DATE '2026-01-01',4.50),(v_proj,DATE '2026-02-01',4.50),
    (v_proj,DATE '2026-03-01',5.00),(v_proj,DATE '2026-04-01',5.00),
    (v_proj,DATE '2026-05-01',5.00),(v_proj,DATE '2026-06-01',4.00),
    (v_proj,DATE '2026-07-01',4.00),(v_proj,DATE '2026-08-01',3.50),
    (v_proj,DATE '2026-09-01',3.00),(v_proj,DATE '2026-10-01',2.50),
    (v_proj,DATE '2026-11-01',2.00),(v_proj,DATE '2026-12-01',2.50);

  -- ─── 8) İSG BULGULARI ───────────────────────────────────────────────────
  INSERT INTO ohs_findings (project_id,subcontractor_id,severity,description,location,
    gps_lat,gps_lng,due_date,status,reported_by,created_at) VALUES
    (v_proj,s_toprak,'Critical','Lastik tekerlekli yükleyici operatörü kemer takmıyor.','Hafriyat Sahası / Km:3',39.9018,32.0041,CURRENT_DATE-3,'Open',v_admin,now()-interval '8 day'),
    (v_proj,s_toprak,'Critical','Kazı yamaçlarında şev güvenliği yetersiz; göçük riski.','Km:5+200 keser',39.9052,32.0078,CURRENT_DATE-1,'InProgress',v_admin,now()-interval '5 day'),
    (v_proj,s_beton,'Major','Beton vibratörü elektrik panosu topraklaması eksik.','Menfez No:7',39.9034,32.0055,CURRENT_DATE+4,'Open',v_admin,now()-interval '4 day'),
    (v_proj,s_kopru,'Major','Köprü iskele altında çalışan ekipte baret yok.','Köprü / Aksı A',39.9065,32.0090,CURRENT_DATE+6,'Open',v_admin,now()-interval '3 day'),
    (v_proj,s_kaplama,'Minor','Asfalt serici yakıt tankı civarında yangın söndürücü bulunmuyor.','Kaplama ekibi / Km:8',39.9080,32.0110,CURRENT_DATE+10,'Open',v_admin,now()-interval '2 day'),
    (v_proj,s_toprak,'Minor','Toz önleme sulama yapılmıyor, civarda yerleşim alanı var.','Şantiye yolu / Km:1',39.8995,32.0020,CURRENT_DATE+14,'InProgress',v_admin,now()-interval '6 day'),
    (v_proj,s_beton,'Observation','İSG eğitim kayıtları 2 işçi için eksik.','Şantiye şefliği',39.9010,32.0035,CURRENT_DATE+20,'Open',v_admin,now()-interval '1 day'),
    (v_proj,s_kopru,'Minor','Geçici korkuluk hattında 3 m boşluk tespit edildi.','Köprü / Aksı B',39.9070,32.0095,CURRENT_DATE-15,'Open',v_admin,now()-interval '22 day');

  UPDATE ohs_findings
     SET status='Closed', closed_by=v_admin, closed_at=now()-interval '12 day',
         close_note='Korkuluk tamamlandı, saha kontrolü yapıldı.'
   WHERE project_id=v_proj AND description LIKE 'Geçici korkuluk%';

  -- ─── 9) MALZEME ONAYLARI ────────────────────────────────────────────────
  INSERT INTO material_approvals (project_id,subcontractor_id,mar_no,material_name,spec_ref,
    manufacturer,status,decision_note,decided_by,decided_at,created_by,created_at) VALUES
    (v_proj,s_toprak,'MAR-K01','Granüler Dolgu Malzemesi (GKD)','KGM/H-Şart.12','İç Kaynak / Ocak','Approved',
     'Granülometri ve CBR deneyleri uygun.',v_admin,now()-interval '90 day',v_admin,now()-interval '95 day'),
    (v_proj,s_beton,'MAR-K02','C30/37 Hazır Beton','TS EN 206-1','Beton Plus A.Ş.','Approved',
     'Numune deneyleri şartnameye uygun; onaylandı.',v_admin,now()-interval '70 day',v_admin,now()-interval '75 day'),
    (v_proj,s_beton,'MAR-K03','B500C Donatı Çeliği','TS 708','Kardemir','Approved',
     'MTC ve deney raporu eksiksiz.',v_admin,now()-interval '65 day',v_admin,now()-interval '70 day'),
    (v_proj,s_kopru,'MAR-K04','Öngerilmeli Tabliye Kirişi (Y1860S7)','TS EN 10138','İnce Betonarme A.Ş.','ConditionallyApproved',
     'Onaylandı; ilk dökümde numune alınacak.',v_admin,now()-interval '30 day',v_admin,now()-interval '35 day'),
    (v_proj,s_kaplama,'MAR-K05','Bitümlü Bağlayıcı PG 70-22','TS EN 12591','Tüpraş','UnderReview',
     NULL,NULL,NULL,v_admin,now()-interval '10 day'),
    (v_proj,s_kaplama,'MAR-K06','Agrega (Bazalt, 0/16 mm)','KGM/H-Şart.18','Polatlı Taş Ocağı','Submitted',
     NULL,NULL,NULL,v_admin,now()-interval '4 day'),
    (v_proj,s_beton,'MAR-K07','Prekast Menfez Çerçevesi (2.0×2.0)','TS 821','Alka Beton','Rejected',
     'Beton sınıfı B25 — proje B30 şartını karşılamıyor; revize teklif bekleniyor.',v_admin,now()-interval '40 day',v_admin,now()-interval '45 day');

  -- ─── 10) GÖREVLER (Kanban) ──────────────────────────────────────────────
  INSERT INTO tasks (project_id,title,description,status,priority,due_date,created_by,kanban_order,created_at) VALUES
    (v_proj,'Km:5+200 şev güvenliği aksiyonu','Kritik bulgu; iksa çalışması başlatılacak.','InProgress','Urgent',CURRENT_DATE+1,v_admin,1,now()-interval '5 day'),
    (v_proj,'5. hakediş kesinleşme takibi','Saha onayı tamam, merkez ofis onayı bekleniyor.','Review','High',CURRENT_DATE+3,v_admin,1,now()-interval '8 day'),
    (v_proj,'Köprü Aksı A–B Çelik Kalıp Temini','Tedarikçi teklifleri değerlendirilecek.','InProgress','High',CURRENT_DATE+7,v_admin,2,now()-interval '12 day'),
    (v_proj,'Toz önleme sulama günlüğü','Her gün saha girişi öncesi sulanacak.','InProgress','Normal',CURRENT_DATE+5,v_admin,3,now()-interval '3 day'),
    (v_proj,'Menfez No:7 pano topraklama tamiri','Elektrikçi çağrılacak.','Todo','Normal',CURRENT_DATE+2,v_admin,1,now()-interval '4 day'),
    (v_proj,'Asfalt bitüm laboratuvar deneyleri','MAR-K05 için tedarikçi numunesi bekleniyor.','Todo','Normal',CURRENT_DATE+9,v_admin,2,now()-interval '7 day'),
    (v_proj,'Aylık ilerleme raporu hazırlama','Temmuz 2025 hakediş verileriyle güncellenecek.','Todo','Normal',CURRENT_DATE+6,v_admin,3,now()-interval '2 day'),
    (v_proj,'Orman arazisi geçiş izin belgesi','Çevre Bakanlığı yazışması takip ediliyor.','Backlog','Low',CURRENT_DATE+45,v_admin,1,now()-interval '30 day'),
    (v_proj,'Köprü aksonometrik as-built çizimi','Proje bitiminde İdare arşivine verilecek.','Backlog','Low',CURRENT_DATE+120,v_admin,2,now()-interval '20 day'),
    (v_proj,'Mobilizasyon kabul tutanağı','İmzalandı, klasöre kaldırıldı.','Done','Normal',CURRENT_DATE-80,v_admin,1,now()-interval '90 day'),
    (v_proj,'Şantiye yolu inşaatı','Tamamlandı; trafiğe açıldı.','Done','High',CURRENT_DATE-60,v_admin,2,now()-interval '70 day'),
    (v_proj,'1. dönem hakediş kesinleşmesi','Kesinleşti, ödeme yapıldı.','Done','Normal',CURRENT_DATE-90,v_admin,3,now()-interval '100 day');

  -- ─── 11) GÜNLÜK SAHA RAPORLARI ──────────────────────────────────────────
  -- Rapor 1
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-4,1,
    '{"condition":"Parçalı Bulutlu","wind_kph":18,"precipitation_mm":0,"source":"manual"}'::jsonb,
    12.0,24.0,'Hafriyat Km:3–5 arası devam. Yükleyici bakımda 2 saat duruş.','Draft',v_admin,now()-interval '4 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_toprak,'Operatör',8),(dr,s_toprak,'İşçi',22),(dr,s_beton,'Kalıpçı',6);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours,idle_reason) VALUES
    (dr,'Lastik Tekerlekli Yükleyici',3,6.0,'2 saatlik bakım duruşu'),(dr,'Damperli Kamyon',8,8.0,NULL),
    (dr,'Ekskavatör (Paletli)',2,8.0,NULL);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_hafriyat,'Km:3+000 – 3+800','Kazı ve hafriyat taşıması',4200.0,'m3'),
    (dr,w_dolgu,'Km:2+500 – 3+200','Granüler dolgu serimi ve sıkıştırma',3100.0,'m3');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '3 day 20 hour', submitted_by=v_admin WHERE id=dr;

  -- Rapor 2
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-3,1,
    '{"condition":"Güneşli","wind_kph":8,"precipitation_mm":0,"source":"manual"}'::jsonb,
    14.0,27.0,'Menfez No:5–6 beton dökümü.','Draft',v_admin,now()-interval '3 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_toprak,'Operatör',6),(dr,s_toprak,'İşçi',18),(dr,s_beton,'Kalıpçı',10),(dr,s_beton,'Demirci',8);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours) VALUES
    (dr,'Ekskavatör (Paletli)',2,8.0),(dr,'Beton Mikseri',4,7.5),(dr,'Kule Vinç',1,8.0);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_menfez,'Menfez No:5','Menfez tabliyesi beton dökümü — C30/37',85.0,'m3'),
    (dr,w_menfez,'Menfez No:6','Menfez yan duvar kalıp ve beton',60.0,'m3');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '2 day 20 hour', submitted_by=v_admin WHERE id=dr;

  -- Rapor 3
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-2,1,
    '{"condition":"Yağışlı","wind_kph":25,"precipitation_mm":12,"source":"manual"}'::jsonb,
    10.0,17.0,'Yağış nedeniyle hafriyat durduruldu. Kalıp işleri devam etti.','Draft',v_admin,now()-interval '2 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_toprak,'İşçi',8),(dr,s_beton,'Kalıpçı',12),(dr,s_beton,'Demirci',10);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours,idle_reason) VALUES
    (dr,'Ekskavatör (Paletli)',2,0.0,'Yağış nedeniyle durduruldu'),(dr,'Kule Vinç',1,7.0,NULL),
    (dr,'Beton Mikseri',2,6.0,NULL);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_menfez,'Menfez No:7','Donatı montajı ve kalıp hazırlığı',0.0,'m3'),
    (dr,w_kopru_beton,'Köprü / Aksı A','Temel kazık başlığı donatı bağlama',12.5,'ton');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '1 day 20 hour', submitted_by=v_admin WHERE id=dr;

  -- Rapor 4 (bugün — taslak)
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-1,1,
    '{"condition":"Güneşli","wind_kph":10,"precipitation_mm":0,"source":"manual"}'::jsonb,
    15.0,28.0,'Normal yoğunlukta çalışma. Km:5+500 hafriyat devam ediyor.','Draft',v_admin,now()-interval '23 hour');
  -- (taslak bırakıldı — henüz gönderilmedi)

  -- ─── 12) SATINALMA — PR ve PO (Tedarikçi Verisi) ───────────────────────
  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-K001',v_admin,CURRENT_DATE+15,'Köprü kalıp sistem kiralama talebi','Draft',now()-interval '22 day')
  RETURNING id INTO pr1;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit,note) VALUES
    (pr1,'Çelik Tünel Kalıp Sistemi (36 m2)','TS 11046 / CE belgeli',4.000,'set','Aksı A–D arası 4 kalıp seti'),
    (pr1,'Kalıp Bağlantı Aksesuarı (takım)','Üretici katalogu',4.000,'takım','Her set için 1 takım');
  UPDATE purchase_requests SET status='Approved', submitted_at=now()-interval '20 day',
    decided_by=v_admin, decided_at=now()-interval '18 day' WHERE id=pr1;

  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-K002',v_admin,CURRENT_DATE+30,'Bitümlü bağlayıcı temin talebi','Draft',now()-interval '10 day')
  RETURNING id INTO pr2;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit) VALUES
    (pr2,'Bitümlü Bağlayıcı PG 70-22','TS EN 12591',120.000,'ton'),
    (pr2,'Polimer Modifiye Bitüm (PMB)','TS EN 14023',40.000,'ton');
  UPDATE purchase_requests SET status='Approved', submitted_at=now()-interval '8 day',
    decided_by=v_admin, decided_at=now()-interval '6 day' WHERE id=pr2;

  -- PO'lar (tedarikçi siparişleri)
  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,
    expected_date,note,created_by,created_at)
  VALUES (v_proj,pr1,'PO-K001','Peri Form Kalıp Sistemleri A.Ş.',485000,'TRY','Delivered',
    CURRENT_DATE-10,'4 set tünel kalıp sistemi teslim alındı.',v_admin,now()-interval '15 day')
  RETURNING id INTO po1;
  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,
    expected_date,note,created_by,created_at)
  VALUES (v_proj,pr2,'PO-K002','Tüpraş Rafineri Ürünleri Tic.',1840000,'TRY','Ordered',
    CURRENT_DATE+25,'120 ton PG 70-22 + 40 ton PMB siparişi verildi.',v_admin,now()-interval '4 day')
  RETURNING id INTO po2;

  RAISE NOTICE 'DEMO-02 verisi oluşturuldu (proje id: %)', v_proj;
END $$;

-- ============================================================================
-- PROJE 2: DEMO-03 — Kent Plaza AVM Yapımı
-- ============================================================================
DO $$
DECLARE
  v_admin   uuid;
  v_role_pm uuid;
  v_proj    uuid;
  s_ber uuid; s_celik uuid; s_cephe uuid; s_mep uuid;
  c_ber uuid; c_celik uuid; c_cephe uuid; c_mep uuid;
  w_kolon uuid; w_kiriş uuid; w_perde uuid; w_celik_kon uuid; w_cephe_cam uuid;
  pp uuid; v_month date; i int; j int;
  gross_cum_arr numeric[] := ARRAY[6500000, 14000000, 22000000, 29500000];
  prev_cum  numeric := 0;
  this_amt  numeric;
  ded_adv   numeric; ded_ret  numeric; ded_wht  numeric;
  ded_ohs   numeric; ded_meal numeric; ded_util numeric;
  v_vat     numeric; v_vat_wh numeric; v_vat_coll numeric;
  v_payable numeric; v_ded_total numeric; v_cost_red numeric; v_actual numeric;
  net_amt   numeric;
  pr1 uuid; po1 uuid; po2 uuid;
  dr uuid;
BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  SELECT id INTO v_role_pm FROM roles WHERE name = 'ProjectManager' LIMIT 1;
  IF v_role_pm IS NULL THEN SELECT id INTO v_role_pm FROM roles ORDER BY created_at LIMIT 1; END IF;

  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-03';
  IF v_proj IS NOT NULL THEN
    UPDATE projects SET code = 'DEMO-03-ARSIV-' || to_char(now(),'YYYYMMDDHH24MISS'),
      name = name || ' (arşiv)', status = 'Archived', deleted_at = now()
    WHERE id = v_proj;
  END IF;

  INSERT INTO projects (code, name, location, client_name, budget_total,
                        currency, start_date, end_date, status)
  VALUES ('DEMO-03', 'Kent Plaza AVM ve Karma Kullanım Yapısı',
          'İzmir / Bayraklı', 'Metropol Gayrimenkul A.Ş.',
          88000000, 'TRY', DATE '2026-01-01', DATE '2027-06-30', 'Active')
  RETURNING id INTO v_proj;
  INSERT INTO project_members (project_id, user_id, role_id) VALUES (v_proj, v_admin, v_role_pm);

  -- Taşeronlar
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Ege Betonarme İnş. A.Ş.','6001112233','Mehmet Can Duman','0532 500 5500','proje@egebeton.com','Betonarme')
  RETURNING id INTO s_ber;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Batı Çelik Konstrüksiyon Ltd.','6002223344','Seda Yıldız','0533 600 6600','info@baticep.com','Çelik Konstrüksiyon')
  RETURNING id INTO s_celik;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Akdeniz Cephe Sistemleri A.Ş.','6003334455','Tolga Kara','0534 700 7700','proje@akdenizcephe.com','Cephe / Giydirme')
  RETURNING id INTO s_cephe;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'İzmir Mekanik Elektrik Ltd.','6004445566','Elif Nur Aslan','0535 800 8800','info@izmirmep.com','MEP (Mekanik–Elektrik)')
  RETURNING id INTO s_mep;

  -- Sözleşmeler
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date,start_date,end_date,is_multi_year,withholding_pct)
  VALUES (v_proj,s_ber,'SZL-2026-A01','Sub',32000000,4800000,5.00,20.00,
    DATE '2025-12-20',DATE '2026-01-01',DATE '2027-06-30',true,5.00)
  RETURNING id INTO c_ber;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date,start_date,end_date,is_multi_year,withholding_pct)
  VALUES (v_proj,s_celik,'SZL-2026-A02','Sub',22000000,3300000,5.00,15.00,
    DATE '2026-02-01',DATE '2026-03-01',DATE '2026-12-31',false,3.00)
  RETURNING id INTO c_celik;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date)
  VALUES (v_proj,s_cephe,'SZL-2026-A03','Sub',18000000,2700000,3.00,10.00,DATE '2026-05-01')
  RETURNING id INTO c_cephe;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date)
  VALUES (v_proj,s_mep,'SZL-2026-A04','Sub',16000000,2400000,3.00,15.00,DATE '2026-01-15')
  RETURNING id INTO c_mep;

  -- Pozlar
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_ber,c_ber,'Y.16.210','Kolon ve perde betonarme (C35/45)','m3',9800,3800.00)
  RETURNING id INTO w_kolon;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_ber,c_ber,'Y.16.215','Döşeme ve kiriş betonarme (C30/37)','m3',14500,2900.00)
  RETURNING id INTO w_kiriş;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_ber,c_ber,'Y.16.205','Perdeler betonarme (C35/45)','m3',5200,4100.00)
  RETURNING id INTO w_perde;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_celik,c_celik,'Y.09.100','HEA/HEB Yapısal çelik montajı','ton',1850,42000.00)
  RETURNING id INTO w_celik_kon;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_cephe,c_cephe,'Y.30.401','Alüminyum giydirme cephe (cam+profil)','m2',12400,1450.00)
  RETURNING id INTO w_cephe_cam;

  -- Hakedişler (4 kesinleşmiş)
  FOR i IN 1..4 LOOP
    v_month  := (DATE '2026-01-01' + ((i-1)||' month')::interval)::date;
    this_amt := gross_cum_arr[i] - prev_cum;
    ded_adv  := round(this_amt * 0.20, 2);
    ded_ret  := round(this_amt * 0.05, 2);
    ded_wht  := round(this_amt * 0.05, 2);
    ded_ohs  := CASE WHEN i IN (1,3) THEN 38000 ELSE 0 END;
    ded_meal := round(this_amt * 0.013, 2);
    ded_util := round(this_amt * 0.008, 2);
    v_vat        := round(this_amt * 0.20, 2);
    v_vat_wh     := round(v_vat * 0.40, 2);
    v_vat_coll   := v_vat - v_vat_wh;
    v_payable    := this_amt + v_vat_coll;
    v_ded_total  := ded_adv + ded_ret + ded_wht + ded_ohs + ded_meal + ded_util;
    net_amt      := v_payable - v_ded_total;
    v_cost_red   := ded_ohs + round(ded_meal/1.10,2) + round(ded_util/1.20,2);
    v_actual     := this_amt - v_cost_red;

    INSERT INTO progress_payments (
      project_id, subcontractor_id, period_no, period_start, period_end, status,
      gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable,
      vat_amount, vat_withheld, vat_collected, payable_gross, actual_cost,
      vat_withholding_ratio, withholding_applied, current_step_no,
      submitted_at, site_approved_at, finalized_at, finalized_by, created_by, created_at)
    VALUES (v_proj, s_ber, i,
      date_trunc('month',v_month)::date,
      (date_trunc('month',v_month)+interval '1 month - 1 day')::date,
      'SiteApproved',
      gross_cum_arr[i], prev_cum, this_amt, v_ded_total, 20.00, net_amt,
      v_vat, v_vat_wh, v_vat_coll, v_payable, v_actual,
      0.40, false, 9,
      (v_month+interval '25 day'),(v_month+interval '27 day'),
      (v_month+interval '28 day'),v_admin,v_admin,(v_month+interval '20 day'))
    RETURNING id INTO pp;

    INSERT INTO payment_deductions
      (progress_payment_id,type,description,rate_pct,amount,nature,reduces_cost,
       group_code,catalog_code,vat_pct,net_amount)
    VALUES
      (pp,'AdvanceOffset','Avans mahsubu',20.00,ded_adv,'Offset',false,'Advance','ADV_CONTRACT',0,ded_adv),
      (pp,'Retention','Teminat kesintisi',5.00,ded_ret,'Temporary',false,'Retention','RET_PERFORMANCE',0,ded_ret),
      (pp,'Withholding','Stopaj (yıllara sari)',5.00,ded_wht,'Permanent',false,'Tax','WHT_INCOME',0,ded_wht),
      (pp,'Other','Öğle yemeği bedeli',NULL,ded_meal,'Permanent',true,'GoodsService','GS_LUNCH',10,round(ded_meal/1.10,2)),
      (pp,'Other','Elektrik ve su bedeli',NULL,ded_util,'Permanent',true,'GoodsService','GS_ELECTRICITY',20,round(ded_util/1.20,2));

    IF ded_ohs > 0 THEN
      INSERT INTO payment_deductions
        (progress_payment_id,type,source_entity,description,amount,nature,reduces_cost,
         group_code,catalog_code,vat_pct,net_amount)
      VALUES (pp,'OHSPenalty','ohs_penalties','İSG ceza tutanağı kesintisi',ded_ohs,'Permanent',true,'Penalty','PEN_OHS',0,ded_ohs);
    END IF;

    INSERT INTO progress_payment_items
      (progress_payment_id,work_item_id,prev_cum_qty,this_period_qty,cum_qty,cum_amount,this_amount)
    VALUES
      (pp,w_kolon,round((prev_cum/3800.0)*0.35,3),round((this_amt/3800.0)*0.35,3),
        round((gross_cum_arr[i]/3800.0)*0.35,3),round(gross_cum_arr[i]*0.35,2),round(this_amt*0.35,2)),
      (pp,w_kiriş,round((prev_cum/2900.0)*0.40,3),round((this_amt/2900.0)*0.40,3),
        round((gross_cum_arr[i]/2900.0)*0.40,3),round(gross_cum_arr[i]*0.40,2),round(this_amt*0.40,2)),
      (pp,w_perde,round((prev_cum/4100.0)*0.25,3),round((this_amt/4100.0)*0.25,3),
        round((gross_cum_arr[i]/4100.0)*0.25,3),round(gross_cum_arr[i]*0.25,2),round(this_amt*0.25,2));

    FOR j IN 1..9 LOOP
      INSERT INTO payment_approvals (progress_payment_id,step_no,step_code,decision,actor_id,created_at)
      SELECT pp,s.step_no,s.code,'Approved',v_admin,(v_month+interval '27 day')+(j||' hour')::interval
      FROM payment_approval_steps s WHERE s.project_id IS NULL AND s.step_no = j;
    END LOOP;

    UPDATE progress_payments SET status = 'Finalized' WHERE id = pp;
    prev_cum := gross_cum_arr[i];
  END LOOP;

  -- 5. dönem — InApproval (zincirde 4. adımda)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,
    vat_amount,vat_withheld,vat_collected,payable_gross,actual_cost,
    vat_withholding_ratio,withholding_applied,current_step_no,
    submitted_at,created_by,created_at)
  VALUES (v_proj,s_ber,5,DATE '2026-05-01',DATE '2026-05-31','InApproval',
    36800000,29500000,7300000,1825000,20.00,5840000,
    1460000,584000,876000,8176000,6800000,0.40,false,4,
    now()-interval '14 day',v_admin,now()-interval '16 day');

  -- 6. dönem — Draft (çelik taşeronu)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,created_by,created_at)
  VALUES (v_proj,s_celik,1,DATE '2026-06-01',DATE '2026-06-30','Draft',
    4900000,0,4900000,1225000,20.00,3675000,v_admin,now()-interval '2 day');

  -- Milestone'lar
  INSERT INTO milestones (project_id,name,planned_date,actual_date,weight_pct,status,sort_order) VALUES
    (v_proj,'Şantiye kurulumu ve zemin etüdü',DATE '2026-01-31',DATE '2026-01-25',4.00,'Completed',1),
    (v_proj,'Temel kazığı ve bodrum betonarme',DATE '2026-04-30',DATE '2026-05-08',20.00,'Completed',2),
    (v_proj,'Normal kat betonarme (+1 - +7)',DATE '2026-08-31',NULL,28.00,'InProgress',3),
    (v_proj,'Çelik çatı konstrüksiyonu ve döşeme',DATE '2026-11-30',NULL,18.00,'Planned',4),
    (v_proj,'Cephe giydirme ve alüminyum doğrama',DATE '2027-02-28',NULL,15.00,'Planned',5),
    (v_proj,'MEP tesisat ve geçici kabul',DATE '2027-06-15',NULL,15.00,'Planned',6);

  -- PV Planı (18 ay)
  INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES
    (v_proj,DATE '2026-01-01',3.00),(v_proj,DATE '2026-02-01',4.00),
    (v_proj,DATE '2026-03-01',5.00),(v_proj,DATE '2026-04-01',6.50),
    (v_proj,DATE '2026-05-01',7.00),(v_proj,DATE '2026-06-01',7.50),
    (v_proj,DATE '2026-07-01',8.00),(v_proj,DATE '2026-08-01',8.00),
    (v_proj,DATE '2026-09-01',7.50),(v_proj,DATE '2026-10-01',7.00),
    (v_proj,DATE '2026-11-01',6.50),(v_proj,DATE '2026-12-01',6.00),
    (v_proj,DATE '2027-01-01',5.50),(v_proj,DATE '2027-02-01',5.00),
    (v_proj,DATE '2027-03-01',4.50),(v_proj,DATE '2027-04-01',4.00),
    (v_proj,DATE '2027-05-01',3.50),(v_proj,DATE '2027-06-01',2.00);

  -- İSG Bulgular
  INSERT INTO ohs_findings (project_id,subcontractor_id,severity,description,location,
    gps_lat,gps_lng,due_date,status,reported_by,created_at) VALUES
    (v_proj,s_ber,'Critical','5. katta dış cephe çalışmasında güvenlik ağı yetersiz; düşme riski.','5. Kat / Kuzey Cephe',38.4592,27.1652,CURRENT_DATE-2,'Open',v_admin,now()-interval '7 day'),
    (v_proj,s_celik,'Critical','Çelik montajında kaynak ekibinde kaynak maskesi kullanılmıyor.','Çelik Atölyesi / Zemin',38.4595,27.1658,CURRENT_DATE-1,'InProgress',v_admin,now()-interval '4 day'),
    (v_proj,s_ber,'Major','Kalıp iskelesi braketi aşırı yüklenmiş, eğilme var.','3. Kat / Aksı C',38.4590,27.1648,CURRENT_DATE+5,'Open',v_admin,now()-interval '5 day'),
    (v_proj,s_mep,'Major','Geçici elektrik tesisatı izolasyon hasarı.','1. Bodrum / Elektrik Odası',38.4588,27.1645,CURRENT_DATE+7,'Open',v_admin,now()-interval '3 day'),
    (v_proj,s_cephe,'Minor','Hava kompresörü basınç tahliye valfi düzgün çalışmıyor.','Cephe Deposu',38.4598,27.1662,CURRENT_DATE+12,'Open',v_admin,now()-interval '2 day'),
    (v_proj,s_ber,'Observation','Taşıyıcı sisteme bağlı olmayan geçici çelik parçalar sahada.','Depo / Arka Bahçe',38.4584,27.1640,CURRENT_DATE+20,'Open',v_admin,now()-interval '1 day'),
    (v_proj,s_celik,'Minor','Malzeme depolama alanında zemin düzensizliği.','Malzeme Deposu / Giriş',38.4601,27.1670,CURRENT_DATE-12,'Open',v_admin,now()-interval '20 day');

  UPDATE ohs_findings
     SET status='Closed', closed_by=v_admin, closed_at=now()-interval '10 day',
         close_note='Depo alanı düzenlendi ve zemin stabilize edildi.'
   WHERE project_id=v_proj AND description LIKE 'Malzeme depolama%';

  -- Malzeme Onayları
  INSERT INTO material_approvals (project_id,subcontractor_id,mar_no,material_name,spec_ref,
    manufacturer,status,decision_note,decided_by,decided_at,created_by,created_at) VALUES
    (v_proj,s_ber,'MAR-A01','C35/45 Hazır Beton','TS EN 206-1','Çimko Beton İzmir','Approved',
     'Basınç deneyleri 7. ve 28. günlerde şartname değerlerini sağladı.',v_admin,now()-interval '80 day',v_admin,now()-interval '85 day'),
    (v_proj,s_ber,'MAR-A02','B500C Donatı Çeliği','TS 708','Kardemir Karabük','Approved',
     'Çekme ve bükme deneyleri uygun; onaylandı.',v_admin,now()-interval '75 day',v_admin,now()-interval '80 day'),
    (v_proj,s_celik,'MAR-A03','HEA 300 Yapısal Çelik Profil','TS EN 10034','ArcelorMittal','Approved',
     'CE belgesi ve MTC kontrolü yapıldı.',v_admin,now()-interval '50 day',v_admin,now()-interval '55 day'),
    (v_proj,s_celik,'MAR-A04','Yüksek Mukavemetli Cıvata M24-10.9','TS EN 14399','Nord-Lock','ConditionallyApproved',
     'Onaylandı; her partide sertlik testi yapılacak.',v_admin,now()-interval '25 day',v_admin,now()-interval '28 day'),
    (v_proj,s_cephe,'MAR-A05','Alüminyum Profil Sistemi 6063-T5','TS EN 12020','Sapa Building Systems','UnderReview',
     NULL,NULL,NULL,v_admin,now()-interval '14 day'),
    (v_proj,s_cephe,'MAR-A06','Lamine Isıcam (4+16+4 LE)','TS EN 1279','Şişecam Flat Glass','Submitted',
     NULL,NULL,NULL,v_admin,now()-interval '6 day'),
    (v_proj,s_mep,'MAR-A07','YJY-0.6/1 kV Kablo (3x185 mm2)','TS IEC 60502-1','Öncab Kablo','Approved',
     'Elektriksel ve mekanik testler tamamlandı.',v_admin,now()-interval '35 day',v_admin,now()-interval '40 day'),
    (v_proj,s_mep,'MAR-A08','Sprinkler Başlığı (68°C / K-80)','TS EN 12259-1','Victaulic','Rejected',
     'Akış katsayısı K-80 projedeki K-115 şartını karşılamıyor; revize teklif bekleniyor.',v_admin,now()-interval '20 day',v_admin,now()-interval '23 day');

  -- Görevler
  INSERT INTO tasks (project_id,title,description,status,priority,due_date,created_by,kanban_order,created_at) VALUES
    (v_proj,'5. kat güvenlik ağı tamamlanması','Kritik İSG bulgusu kapatılacak.','InProgress','Urgent',CURRENT_DATE+1,v_admin,1,now()-interval '7 day'),
    (v_proj,'5. dönem hakediş onay zinciri takibi','4. adımda bekliyor; şantiye müdürü onayı alınacak.','Review','High',CURRENT_DATE+3,v_admin,1,now()-interval '14 day'),
    (v_proj,'Çelik kaynak kayıt defteri güncelleme','6 kaynak noktası kaydı eksik.','InProgress','High',CURRENT_DATE+4,v_admin,2,now()-interval '4 day'),
    (v_proj,'Sprinkler MAR revize teklifi değerlendirme','Victaulic K-115 teklifi bekleniyor.','InProgress','Normal',CURRENT_DATE+8,v_admin,3,now()-interval '10 day'),
    (v_proj,'Alüminyum cephe profil MAR incelemesi','Teknik föy analizi.','Todo','Normal',CURRENT_DATE+5,v_admin,1,now()-interval '8 day'),
    (v_proj,'4. kat kalıp kontrol raporu','Kalıp sökümü öncesi statik kontrol.','Todo','Normal',CURRENT_DATE+6,v_admin,2,now()-interval '3 day'),
    (v_proj,'Aylık İSG toplantısı tutanağı','Nisan bulguları gündeme alınacak.','Todo','Normal',CURRENT_DATE+9,v_admin,3,now()-interval '2 day'),
    (v_proj,'Asansör pit betonarme revizyonu','İdare onaylı revize proje uygulanacak.','Backlog','Normal',CURRENT_DATE+25,v_admin,1,now()-interval '15 day'),
    (v_proj,'Çatı çelik montaj takvim güncellemesi','Gecikme değerlendirmesi yapılacak.','Backlog','Low',CURRENT_DATE+35,v_admin,2,now()-interval '12 day'),
    (v_proj,'Zemin etüdü raporu arşivleme','Tamamlandı, EKAP dokümanına eklendi.','Done','Normal',CURRENT_DATE-60,v_admin,1,now()-interval '70 day'),
    (v_proj,'Bodrum betonarme 1. kat kabul tutanağı','İmzalandı, dosyalandı.','Done','High',CURRENT_DATE-35,v_admin,2,now()-interval '40 day'),
    (v_proj,'1. dönem hakediş kesinleşmesi','Kesinleşti, ödeme transferi yapıldı.','Done','Normal',CURRENT_DATE-50,v_admin,3,now()-interval '55 day');

  -- Günlük Saha Raporları
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-5,1,
    '{"condition":"Güneşli","wind_kph":12,"precipitation_mm":0,"source":"manual"}'::jsonb,
    18.0,30.0,'5. kat güney aksı döşeme kalıpları kuruluyor. Çelik H-kirişler depodan çıkarıldı.','Draft',v_admin,now()-interval '5 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_ber,'Kalıpçı',14),(dr,s_ber,'Demirci',10),(dr,s_celik,'Kaynakçı',6),(dr,s_celik,'Montajcı',4);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours) VALUES
    (dr,'Tower Kren (TC-5510)',1,9.0),(dr,'Beton Pompası',1,7.0),(dr,'Hidrolik Makas',2,8.0);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_kolon,'5. Kat / Aksı A–D','Kolon ve perde betonarme kalıp montajı',0.0,'m3'),
    (dr,w_celik_kon,'5. Kat / Aksı E–G','HEA 300 kiriş montajı',22.5,'ton');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '4 day 20 hour', submitted_by=v_admin WHERE id=dr;

  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-3,1,
    '{"condition":"Az Bulutlu","wind_kph":15,"precipitation_mm":0,"source":"manual"}'::jsonb,
    17.0,29.0,'Döşeme beton dökümü tamamlandı. Cephe ekibi iskele kurulumu yaptı.','Draft',v_admin,now()-interval '3 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_ber,'Kalıpçı',12),(dr,s_ber,'Demirci',8),(dr,s_cephe,'Cephe İşçisi',8);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours) VALUES
    (dr,'Tower Kren (TC-5510)',1,9.5),(dr,'Beton Mikseri',3,8.0),(dr,'Makaslı Platform (9 m)',2,8.0);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_kiriş,'5. Kat / Aksı A–G','5. kat döşeme beton dökümü C30/37',380.0,'m3'),
    (dr,w_cephe_cam,'Zemin Kat / Güney Cephe','Cephe iskele kurulumu ve ankraj',0.0,'m2');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '2 day 20 hour', submitted_by=v_admin WHERE id=dr;

  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-1,1,
    '{"condition":"Güneşli","wind_kph":8,"precipitation_mm":0,"source":"manual"}'::jsonb,
    19.0,32.0,'6. kat perde duvar kaliplari kuruluyor. MEP kanallari B1 bodrum katinda doseniyor.','Draft',v_admin,now()-interval '1 day');
  -- (taslak bırakıldı)

  -- Satınalma
  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-A001',v_admin,CURRENT_DATE+20,'Cephe alüminyum profil sistemi temin talebi','Draft',now()-interval '17 day')
  RETURNING id INTO pr1;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit,note) VALUES
    (pr1,'Aluminyum Profil 6063-T5','TS EN 12020 / Anodize',9800.000,'m','Cephe dikey ve yatay profilleri'),
    (pr1,'Lamine Isicam Cam 4+16+4','TS EN 1279 / Low-E',7200.000,'m2','Guney ve kuzey cepheler');
  UPDATE purchase_requests SET status='Approved', submitted_at=now()-interval '15 day',
    decided_by=v_admin, decided_at=now()-interval '13 day' WHERE id=pr1;

  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,expected_date,note,created_by,created_at)
  VALUES (v_proj,pr1,'PO-A001','Sapa Building Systems Türkiye A.Ş.',1420000,'TRY','PartiallyDelivered',
    CURRENT_DATE+15,'İlk partide 4000 m profil teslim alındı; 2. parti bekleniyor.',v_admin,now()-interval '10 day')
  RETURNING id INTO po1;
  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,expected_date,note,created_by,created_at)
  VALUES (v_proj,NULL,'PO-A002','Şişecam Flat Glass A.Ş.',1044000,'TRY','Ordered',
    CURRENT_DATE+30,'7200 m2 lamine ısıcam siparişi verildi.',v_admin,now()-interval '5 day')
  RETURNING id INTO po2;

  RAISE NOTICE 'DEMO-03 verisi oluşturuldu (proje id: %)', v_proj;
END $$;

-- ============================================================================
-- PROJE 3: DEMO-04 — Şehir Hastanesi Kompleksi
-- ============================================================================
DO $$
DECLARE
  v_admin   uuid;
  v_role_pm uuid;
  v_proj    uuid;
  s_kaba uuid; s_tibbi uuid; s_elektrik uuid; s_hvac uuid;
  c_kaba uuid; c_tibbi uuid; c_elektrik uuid; c_hvac uuid;
  w_temel uuid; w_kat uuid; w_tibbi_gaz uuid; w_elektrik_pano uuid; w_klima uuid;
  pp uuid; v_month date; i int; j int;
  gross_cum_arr numeric[] := ARRAY[18000000, 38000000, 60000000, 82000000, 105000000, 126000000];
  prev_cum  numeric := 0;
  this_amt  numeric;
  ded_adv   numeric; ded_ret  numeric; ded_wht  numeric;
  ded_ohs   numeric; ded_meal numeric; ded_util numeric;
  v_vat     numeric; v_vat_wh numeric; v_vat_coll numeric;
  v_payable numeric; v_ded_total numeric; v_cost_red numeric; v_actual numeric;
  net_amt   numeric;
  pr1 uuid; pr2 uuid; pr3 uuid; po1 uuid; po2 uuid;
  dr uuid;
BEGIN
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  SELECT id INTO v_role_pm FROM roles WHERE name = 'ProjectManager' LIMIT 1;
  IF v_role_pm IS NULL THEN SELECT id INTO v_role_pm FROM roles ORDER BY created_at LIMIT 1; END IF;

  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-04';
  IF v_proj IS NOT NULL THEN
    UPDATE projects SET code = 'DEMO-04-ARSIV-' || to_char(now(),'YYYYMMDDHH24MISS'),
      name = name || ' (arşiv)', status = 'Archived', deleted_at = now()
    WHERE id = v_proj;
  END IF;

  INSERT INTO projects (code, name, location, client_name, budget_total,
                        currency, start_date, end_date, status)
  VALUES ('DEMO-04', 'Nilüfer Şehir Hastanesi Kompleksi',
          'Bursa / Nilüfer', 'Türkiye Kamu Hastaneleri Kurumu (TKHK)',
          320000000, 'TRY', DATE '2025-01-01', DATE '2027-12-31', 'Active')
  RETURNING id INTO v_proj;
  INSERT INTO project_members (project_id, user_id, role_id) VALUES (v_proj, v_admin, v_role_pm);

  -- Taşeronlar
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Marmara Kaba Yapı ve İnş. A.Ş.','7001112233','Ahmet Rıza Yılmaz','0532 900 9900','proje@marmarakaba.com','Kaba Yapı')
  RETURNING id INTO s_kaba;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Medikal Tesisat Sistemleri A.Ş.','7002223344','Dr. Canan Koç','0533 800 8800','proje@medikal-tesisat.com','Tıbbi Gaz ve Tesisat')
  RETURNING id INTO s_tibbi;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Bursa Güç Elektrik Taahhüt Ltd.','7003334455','Ozan Demir','0534 700 7700','info@bursaguc.com','Elektrik / Güç')
  RETURNING id INTO s_elektrik;
  INSERT INTO subcontractors (project_id,company_name,tax_no,contact_person,phone,email,trade)
  VALUES (v_proj,'Klima-Pro Havalandırma A.Ş.','7004445566','Pınar Altun','0535 600 6600','proje@klimapro.com','HVAC / Havalandırma')
  RETURNING id INTO s_hvac;

  -- Sözleşmeler
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date,start_date,end_date,is_multi_year,withholding_pct)
  VALUES (v_proj,s_kaba,'SZL-2025-H01','Sub',135000000,20250000,5.00,20.00,
    DATE '2024-12-01',DATE '2025-01-01',DATE '2027-06-30',true,5.00)
  RETURNING id INTO c_kaba;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date,start_date,end_date,is_multi_year,withholding_pct)
  VALUES (v_proj,s_tibbi,'SZL-2025-H02','Sub',68000000,10200000,5.00,10.00,
    DATE '2025-03-01',DATE '2025-06-01',DATE '2027-12-31',true,5.00)
  RETURNING id INTO c_tibbi;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date,start_date,end_date,is_multi_year,withholding_pct)
  VALUES (v_proj,s_elektrik,'SZL-2025-H03','Sub',62000000,9300000,5.00,15.00,
    DATE '2025-02-01',DATE '2025-04-01',DATE '2027-10-31',true,5.00)
  RETURNING id INTO c_elektrik;
  INSERT INTO contracts (project_id,subcontractor_id,contract_no,type,amount,advance_amount,
    retention_pct,advance_rate_pct,sign_date)
  VALUES (v_proj,s_hvac,'SZL-2025-H04','Sub',55000000,8250000,3.00,10.00,DATE '2025-02-15')
  RETURNING id INTO c_hvac;

  -- Pozlar
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaba,c_kaba,'Y.16.001','Temel ve bodrum betonarme (C35/45)','m3',28000,4500.00)
  RETURNING id INTO w_temel;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaba,c_kaba,'Y.16.210','Normal kat betonarme (C30/37)','m3',52000,3200.00)
  RETURNING id INTO w_kat;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_tibbi,c_tibbi,'Y.M01','Tıbbi gaz sistemi (O2/CO2/N2O/Vakum)','nokta',3200,18500.00)
  RETURNING id INTO w_tibbi_gaz;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_elektrik,c_elektrik,'Y.E01','Ana dağıtım panosu ve besleme hattı','adet',48,85000.00)
  RETURNING id INTO w_elektrik_pano;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_hvac,c_hvac,'Y.H01','Merkezi klima santrali ve kanal sistemi','m2',38000,1450.00)
  RETURNING id INTO w_klima;

  -- Hakedişler (6 kesinleşmiş)
  FOR i IN 1..6 LOOP
    v_month  := (DATE '2025-01-01' + ((i-1)||' month')::interval)::date;
    this_amt := gross_cum_arr[i] - prev_cum;
    ded_adv  := round(this_amt * 0.20, 2);
    ded_ret  := round(this_amt * 0.05, 2);
    ded_wht  := round(this_amt * 0.05, 2);
    ded_ohs  := CASE WHEN i IN (2,4,6) THEN 75000 ELSE 0 END;
    ded_meal := round(this_amt * 0.015, 2);
    ded_util := round(this_amt * 0.010, 2);
    v_vat        := round(this_amt * 0.20, 2);
    v_vat_wh     := round(v_vat * 0.40, 2);
    v_vat_coll   := v_vat - v_vat_wh;
    v_payable    := this_amt + v_vat_coll;
    v_ded_total  := ded_adv + ded_ret + ded_wht + ded_ohs + ded_meal + ded_util;
    net_amt      := v_payable - v_ded_total;
    v_cost_red   := ded_ohs + round(ded_meal/1.10,2) + round(ded_util/1.20,2);
    v_actual     := this_amt - v_cost_red;

    INSERT INTO progress_payments (
      project_id, subcontractor_id, period_no, period_start, period_end, status,
      gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable,
      vat_amount, vat_withheld, vat_collected, payable_gross, actual_cost,
      vat_withholding_ratio, withholding_applied, current_step_no,
      submitted_at, site_approved_at, finalized_at, finalized_by, created_by, created_at)
    VALUES (v_proj, s_kaba, i,
      date_trunc('month',v_month)::date,
      (date_trunc('month',v_month)+interval '1 month - 1 day')::date,
      'SiteApproved',
      gross_cum_arr[i], prev_cum, this_amt, v_ded_total, 20.00, net_amt,
      v_vat, v_vat_wh, v_vat_coll, v_payable, v_actual,
      0.40, true, 9,
      (v_month+interval '25 day'),(v_month+interval '27 day'),
      (v_month+interval '28 day'),v_admin,v_admin,(v_month+interval '20 day'))
    RETURNING id INTO pp;

    INSERT INTO payment_deductions
      (progress_payment_id,type,description,rate_pct,amount,nature,reduces_cost,
       group_code,catalog_code,vat_pct,net_amount)
    VALUES
      (pp,'AdvanceOffset','Avans mahsubu',20.00,ded_adv,'Offset',false,'Advance','ADV_CONTRACT',0,ded_adv),
      (pp,'Retention','Teminat kesintisi',5.00,ded_ret,'Temporary',false,'Retention','RET_PERFORMANCE',0,ded_ret),
      (pp,'Withholding','Stopaj (yıllara sari)',5.00,ded_wht,'Permanent',false,'Tax','WHT_INCOME',0,ded_wht),
      (pp,'Other','Öğle yemeği bedeli',NULL,ded_meal,'Permanent',true,'GoodsService','GS_LUNCH',10,round(ded_meal/1.10,2)),
      (pp,'Other','Elektrik ve su bedeli',NULL,ded_util,'Permanent',true,'GoodsService','GS_ELECTRICITY',20,round(ded_util/1.20,2));

    IF ded_ohs > 0 THEN
      INSERT INTO payment_deductions
        (progress_payment_id,type,source_entity,description,amount,nature,reduces_cost,
         group_code,catalog_code,vat_pct,net_amount)
      VALUES (pp,'OHSPenalty','ohs_penalties','İSG ceza tutanağı kesintisi',ded_ohs,'Permanent',true,'Penalty','PEN_OHS',0,ded_ohs);
    END IF;

    INSERT INTO progress_payment_items
      (progress_payment_id,work_item_id,prev_cum_qty,this_period_qty,cum_qty,cum_amount,this_amount)
    VALUES
      (pp,w_temel,round((prev_cum/4500.0)*0.35,3),round((this_amt/4500.0)*0.35,3),
        round((gross_cum_arr[i]/4500.0)*0.35,3),round(gross_cum_arr[i]*0.35,2),round(this_amt*0.35,2)),
      (pp,w_kat,round((prev_cum/3200.0)*0.40,3),round((this_amt/3200.0)*0.40,3),
        round((gross_cum_arr[i]/3200.0)*0.40,3),round(gross_cum_arr[i]*0.40,2),round(this_amt*0.40,2)),
      (pp,w_tibbi_gaz,round((prev_cum/18500.0)*0.25,3),round((this_amt/18500.0)*0.25,3),
        round((gross_cum_arr[i]/18500.0)*0.25,3),round(gross_cum_arr[i]*0.25,2),round(this_amt*0.25,2));

    FOR j IN 1..9 LOOP
      INSERT INTO payment_approvals (progress_payment_id,step_no,step_code,decision,actor_id,created_at)
      SELECT pp,s.step_no,s.code,'Approved',v_admin,(v_month+interval '27 day')+(j||' hour')::interval
      FROM payment_approval_steps s WHERE s.project_id IS NULL AND s.step_no = j;
    END LOOP;

    UPDATE progress_payments SET status = 'Finalized' WHERE id = pp;
    prev_cum := gross_cum_arr[i];
  END LOOP;

  -- 7. dönem — SiteApproved (onay bekliyor)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,
    vat_amount,vat_withheld,vat_collected,payable_gross,actual_cost,
    vat_withholding_ratio,withholding_applied,current_step_no,
    submitted_at,site_approved_at,created_by,created_at)
  VALUES (v_proj,s_kaba,7,DATE '2025-07-01',DATE '2025-07-31','SiteApproved',
    144500000,126000000,18500000,4625000,20.00,14800000,
    3700000,1480000,2220000,20720000,17200000,0.40,true,7,
    now()-interval '12 day',now()-interval '9 day',v_admin,now()-interval '14 day');

  -- 8. dönem — Draft (elektrik taşeronu)
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,created_by,created_at)
  VALUES (v_proj,s_elektrik,1,DATE '2025-07-01',DATE '2025-07-31','Draft',
    9800000,0,9800000,2450000,20.00,7350000,v_admin,now()-interval '4 day');

  -- Milestone'lar
  INSERT INTO milestones (project_id,name,planned_date,actual_date,weight_pct,status,sort_order) VALUES
    (v_proj,'Şantiye kurulumu ve şev stabilizasyonu',DATE '2025-02-28',DATE '2025-02-22',3.00,'Completed',1),
    (v_proj,'Temel ve bodrum betonarme (B1–B3)',DATE '2025-07-31',DATE '2025-08-15',18.00,'Completed',2),
    (v_proj,'Kaba yapı — zemin ve normal katlar (+1–+8)',DATE '2026-04-30',NULL,28.00,'InProgress',3),
    (v_proj,'Çatı ve üst blok betonarme',DATE '2026-10-31',NULL,16.00,'Planned',4),
    (v_proj,'Tıbbi gaz, elektrik ve HVAC tesisat',DATE '2027-06-30',NULL,22.00,'Planned',5),
    (v_proj,'İnce işler, tıbbi ekipman yerleşimi ve geçici kabul',DATE '2027-12-01',NULL,13.00,'Planned',6);

  -- PV Planı (36 ay)
  INSERT INTO pv_plan_entries (project_id, month, planned_pct) VALUES
    (v_proj,DATE '2025-01-01',1.50),(v_proj,DATE '2025-02-01',2.00),
    (v_proj,DATE '2025-03-01',2.50),(v_proj,DATE '2025-04-01',3.00),
    (v_proj,DATE '2025-05-01',3.50),(v_proj,DATE '2025-06-01',3.50),
    (v_proj,DATE '2025-07-01',4.00),(v_proj,DATE '2025-08-01',4.00),
    (v_proj,DATE '2025-09-01',4.00),(v_proj,DATE '2025-10-01',4.00),
    (v_proj,DATE '2025-11-01',3.50),(v_proj,DATE '2025-12-01',3.50),
    (v_proj,DATE '2026-01-01',3.50),(v_proj,DATE '2026-02-01',3.50),
    (v_proj,DATE '2026-03-01',4.00),(v_proj,DATE '2026-04-01',4.00),
    (v_proj,DATE '2026-05-01',4.00),(v_proj,DATE '2026-06-01',4.00),
    (v_proj,DATE '2026-07-01',3.50),(v_proj,DATE '2026-08-01',3.50),
    (v_proj,DATE '2026-09-01',3.50),(v_proj,DATE '2026-10-01',3.00),
    (v_proj,DATE '2026-11-01',3.00),(v_proj,DATE '2026-12-01',3.00),
    (v_proj,DATE '2027-01-01',3.00),(v_proj,DATE '2027-02-01',2.50),
    (v_proj,DATE '2027-03-01',2.50),(v_proj,DATE '2027-04-01',2.50),
    (v_proj,DATE '2027-05-01',2.50),(v_proj,DATE '2027-06-01',2.00),
    (v_proj,DATE '2027-07-01',2.00),(v_proj,DATE '2027-08-01',2.00),
    (v_proj,DATE '2027-09-01',1.50),(v_proj,DATE '2027-10-01',1.50),
    (v_proj,DATE '2027-11-01',1.00),(v_proj,DATE '2027-12-01',1.00);

  -- İSG Bulgular
  INSERT INTO ohs_findings (project_id,subcontractor_id,severity,description,location,
    gps_lat,gps_lng,due_date,status,reported_by,created_at) VALUES
    (v_proj,s_kaba,'Critical','7. kat güney perdesinde kalıp baskı fisürü — acil boşaltma.','7. Kat / Perde No:S-12',40.2010,29.0482,CURRENT_DATE-2,'Open',v_admin,now()-interval '3 day'),
    (v_proj,s_tibbi,'Critical','O2 besleme hattında basınç kaçağı tespit edildi.','B2 Kat / Tıbbi Gaz Odası',40.2005,29.0475,CURRENT_DATE,  'InProgress',v_admin,now()-interval '1 day'),
    (v_proj,s_kaba,'Major','Yük asansörü emniyet kilidinde arıza; kullanım durduruldu.','Yük Asansörü No:2',40.2012,29.0486,CURRENT_DATE+3,'Open',v_admin,now()-interval '4 day'),
    (v_proj,s_elektrik,'Major','Geçici jeneratör egzost borusu çalışma alanına yönlü.','Şantiye Jeneratör Alanı',40.2000,29.0468,CURRENT_DATE+6,'Open',v_admin,now()-interval '5 day'),
    (v_proj,s_hvac,'Major','Havalandırma kanalı montajında iskele bağlantısı güvensiz.','5. Kat / HVAC Şaftı',40.2018,29.0495,CURRENT_DATE+8,'Open',v_admin,now()-interval '6 day'),
    (v_proj,s_kaba,'Minor','Bodrum kat yangın söndürücü dolumu gerekiyor.','B3 Kat / Koridorlar',40.2003,29.0471,CURRENT_DATE+15,'Open',v_admin,now()-interval '2 day'),
    (v_proj,s_tibbi,'Minor','Tıbbi gaz boru kaynak kayıtları 3 noktada eksik.','B1 Kat / Hat A',40.2007,29.0478,CURRENT_DATE+18,'InProgress',v_admin,now()-interval '7 day'),
    (v_proj,s_elektrik,'Observation','Güç kablosu geçici binada yerden 30 cm yükseklikte.','Şantiye Ofisi / Giriş',40.1998,29.0465,CURRENT_DATE+25,'Open',v_admin,now()-interval '1 day'),
    (v_proj,s_kaba,'Minor','Çelik donatı depolama alanında nem birikmesi.','Açık Depo / Kuzeybatı',40.2022,29.0502,CURRENT_DATE-20,'Open',v_admin,now()-interval '28 day');

  UPDATE ohs_findings
     SET status='Closed', closed_by=v_admin, closed_at=now()-interval '18 day',
         close_note='Depo üzeri naylon örtü ve drenaj kanalı yapıldı.'
   WHERE project_id=v_proj AND description LIKE 'Çelik donatı depolama%';

  -- Malzeme Onayları
  INSERT INTO material_approvals (project_id,subcontractor_id,mar_no,material_name,spec_ref,
    manufacturer,status,decision_note,decided_by,decided_at,created_by,created_at) VALUES
    (v_proj,s_kaba,'MAR-H01','C35/45 Hazır Beton','TS EN 206-1','Limak Beton Bursa','Approved',
     '28 günlük basınç değerleri şartnameyi sağlıyor.',v_admin,now()-interval '120 day',v_admin,now()-interval '125 day'),
    (v_proj,s_kaba,'MAR-H02','B500C Donatı Çeliği (Nervürlü)','TS 708','Erdemir','Approved',
     'İçdaş ve Erdemir MTC karşılaştırması yapıldı; onaylandı.',v_admin,now()-interval '110 day',v_admin,now()-interval '115 day'),
    (v_proj,s_tibbi,'MAR-H03','Tıbbi Oksijen Gaz Boru (316L SS)','EN 13480 / HTM 02-01','Sandvik','Approved',
     'Temizlik ve pasivizasyon sertifikası alındı.',v_admin,now()-interval '80 day',v_admin,now()-interval '85 day'),
    (v_proj,s_tibbi,'MAR-H04','Tıbbi Vakum Kompresörü (Oil-Free)','HTM 02-01 / EN ISO 10079','Atlas Copco','ConditionallyApproved',
     'Onaylandı; FAT (fabrika kabul testi) belgesi teslim alınacak.',v_admin,now()-interval '40 day',v_admin,now()-interval '43 day'),
    (v_proj,s_elektrik,'MAR-H05','Ana Dağıtım Panosu (MDB-3200A)','IEC 61439-1','Schneider Electric','Approved',
     'Onaylandı; fabrika FAT protokolü incelendi.',v_admin,now()-interval '60 day',v_admin,now()-interval '65 day'),
    (v_proj,s_elektrik,'MAR-H06','Jeneratör (3×1000 kVA)','ISO 8528 / CE','Caterpillar','UnderReview',
     NULL,NULL,NULL,v_admin,now()-interval '18 day'),
    (v_proj,s_hvac,'MAR-H07','Hava Sağlama Santrali (AHU – 25000 m³/h)','EN 1886 / VDI 6022','Trane HVAC','Submitted',
     NULL,NULL,NULL,v_admin,now()-interval '7 day'),
    (v_proj,s_kaba,'MAR-H08','Perdeli Perde Beton Kalıp (Alüminyum)','Üretici teknik föyü','Efco','Rejected',
     'Yük kapasitesi 60 kN/m² — proje 75 kN/m² gerektiriyor; revize teklif bekleniyor.',v_admin,now()-interval '30 day',v_admin,now()-interval '33 day');

  -- Görevler
  INSERT INTO tasks (project_id,title,description,status,priority,due_date,created_by,kanban_order,created_at) VALUES
    (v_proj,'7. kat perde fisürü acil müdahalesi','Kalıp boşaltıldı; statik hesap revizesi alınacak.','InProgress','Urgent',CURRENT_DATE,v_admin,1,now()-interval '3 day'),
    (v_proj,'O2 hattı basınç kaçağı giderilmesi','Tıbbi gaz ekibi sahaya çağrıldı.','InProgress','Urgent',CURRENT_DATE+1,v_admin,2,now()-interval '1 day'),
    (v_proj,'7. dönem hakediş kesinleştirme','Saha onayı tamam; finansal kontrol adımında.','Review','High',CURRENT_DATE+4,v_admin,1,now()-interval '12 day'),
    (v_proj,'Jeneratör MAR teknik değerlendirme','Caterpillar ve Perkins teknik karşılaştırma.','InProgress','High',CURRENT_DATE+8,v_admin,3,now()-interval '10 day'),
    (v_proj,'Yük asansörü emniyet kilidi tamiri','Tamir firması çağrıldı.','Todo','Normal',CURRENT_DATE+2,v_admin,1,now()-interval '4 day'),
    (v_proj,'Tıbbi gaz kaynak kayıtları tamamlama','B1 kat Hat A, 3 nokta eksik.','Todo','Normal',CURRENT_DATE+5,v_admin,2,now()-interval '7 day'),
    (v_proj,'HVAC AHU MAR incelemesi','Trane teknik föyü ve enerji etiketi kontrolü.','Todo','Normal',CURRENT_DATE+9,v_admin,3,now()-interval '5 day'),
    (v_proj,'Aylık İSG denetim raporu','Temmuz 2025 sahası 9 bulgu — dağıtım yapılacak.','Todo','Normal',CURRENT_DATE+7,v_admin,1,now()-interval '2 day'),
    (v_proj,'Elektrik şaft yangın bariyer montajı','ETA: Ekim 2025 — planlamaya alındı.','Backlog','Normal',CURRENT_DATE+60,v_admin,1,now()-interval '20 day'),
    (v_proj,'Radyoloji birimi kurşun kaplama kontrol','Mimar onaylı çizim hazırlanacak.','Backlog','Low',CURRENT_DATE+90,v_admin,2,now()-interval '15 day'),
    (v_proj,'Mobilizasyon ve şantiye çevre düzenlemesi','Tamamlandı; idare tarafından onaylandı.','Done','Normal',CURRENT_DATE-130,v_admin,1,now()-interval '140 day'),
    (v_proj,'1.–3. dönem hakedişler kesinleşmesi','Tümü kesinleşti ve ödeme yapıldı.','Done','High',CURRENT_DATE-60,v_admin,2,now()-interval '65 day');

  -- Günlük Saha Raporları (5 adet)
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-6,1,
    '{"condition":"Güneşli","wind_kph":10,"precipitation_mm":0,"source":"manual"}'::jsonb,
    16.0,29.0,'B3 kat plak dökümü tamamlandı. 7. kat perde kalıp kuruluyor.','Draft',v_admin,now()-interval '6 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_kaba,'Kalıpçı',20),(dr,s_kaba,'Demirci',18),(dr,s_kaba,'Beton İşçisi',14),
    (dr,s_tibbi,'Tesisat Ustası',8),(dr,s_elektrik,'Elektrikçi',10);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours) VALUES
    (dr,'Tower Kren (TC-7034)',2,9.5),(dr,'Beton Pompası',2,8.0),
    (dr,'Beton Mikseri',5,8.5),(dr,'Vibratör Takımı',8,8.0);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_temel,'B3 Kat / Aksı D–H','Temel plak beton dökümü C35/45',420.0,'m3'),
    (dr,w_kat,'7. Kat / Aksı A–D','Perde kalıp ve donatı montajı',0.0,'m3'),
    (dr,w_tibbi_gaz,'B1 Kat / Hat B','Tıbbi gaz bakır boru kaynağı',28.0,'nokta');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '5 day 20 hour', submitted_by=v_admin WHERE id=dr;

  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-4,1,
    '{"condition":"Parçalı Bulutlu","wind_kph":20,"precipitation_mm":0,"source":"manual"}'::jsonb,
    15.0,26.0,'Rüzgarlı hava nedeniyle vinç çalışması öğle saatlerinde yavaşladı.','Draft',v_admin,now()-interval '4 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_kaba,'Kalıpçı',18),(dr,s_kaba,'Demirci',16),(dr,s_elektrik,'Elektrikçi',12),(dr,s_hvac,'Kanal Ustası',6);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours,idle_reason) VALUES
    (dr,'Tower Kren (TC-7034)',2,6.0,'Rüzgar hızı 55 km/h — öğle arası durduruldu'),(dr,'Beton Pompası',1,8.0,NULL);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_kat,'7. Kat / Aksı A–D','7. kat perde betonarme donatı bağlama',18.5,'ton'),
    (dr,w_klima,'B2 Kat / HVAC Şaftı','Havalandırma ana kanalı asma montajı',0.0,'m2');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '3 day 20 hour', submitted_by=v_admin WHERE id=dr;

  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-3,1,
    '{"condition":"Güneşli","wind_kph":8,"precipitation_mm":0,"source":"manual"}'::jsonb,
    17.0,31.0,'7. kat perde betonarme dökümü.','Draft',v_admin,now()-interval '3 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_kaba,'Kalıpçı',22),(dr,s_kaba,'Demirci',15),(dr,s_kaba,'Beton İşçisi',12),(dr,s_tibbi,'Tesisat Ustası',8);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours) VALUES
    (dr,'Tower Kren (TC-7034)',2,9.0),(dr,'Beton Pompası',2,8.5),(dr,'Beton Mikseri',6,8.0);
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_kat,'7. Kat / Perde S-01–S-16','Perde ve kolon beton dökümü C35/45',285.0,'m3'),
    (dr,w_tibbi_gaz,'B2 Kat / Hat C','O2/N2O boru montajı ve kaynak',35.0,'nokta');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '2 day 20 hour', submitted_by=v_admin WHERE id=dr;

  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-2,1,
    '{"condition":"Güneşli","wind_kph":5,"precipitation_mm":0,"source":"manual"}'::jsonb,
    18.0,32.0,'Kritik güvenlik olayı: 7. kat perde fisürü tespit edildi. Kalıp boşaltıldı.','Draft',v_admin,now()-interval '2 day')
  RETURNING id INTO dr;
  INSERT INTO daily_manpower (daily_report_id,subcontractor_id,trade,headcount) VALUES
    (dr,s_kaba,'Kalıpçı',14),(dr,s_kaba,'Demirci',10),(dr,s_kaba,'İSG Uzmanı',2);
  INSERT INTO daily_equipment (daily_report_id,equipment_name,count,working_hours,idle_reason) VALUES
    (dr,'Tower Kren (TC-7034)',2,4.0,'Güvenlik önlemi — erken durduruldu'),(dr,'Beton Pompası',1,0.0,'Acil durum');
  INSERT INTO daily_work_entries (daily_report_id,work_item_id,location,description,qty,unit) VALUES
    (dr,w_kat,'7. Kat / Perde S-12','Fisür tespiti — statik mühendis çağrıldı, kalıp boşaltıldı',0.0,'m3'),
    (dr,w_elektrik_pano,'B2 Kat / Elektrik Odası','MDB No:3 kablaj çalışmaları',0.0,'adet');
  UPDATE daily_reports SET status='Submitted', submitted_at=now()-interval '1 day 20 hour', submitted_by=v_admin WHERE id=dr;

  -- Son rapor — taslak
  INSERT INTO daily_reports (project_id,report_date,revision_no,weather,temperature_min,temperature_max,
    notes,status,author_id,created_at)
  VALUES (v_proj,CURRENT_DATE-1,1,
    '{"condition":"Güneşli","wind_kph":7,"precipitation_mm":0,"source":"manual"}'::jsonb,
    19.0,31.0,'Statik mühendis sahada inceleme yaptı. Güçlendirme planı hazırlanıyor.','Draft',v_admin,now()-interval '23 hour');
  -- (taslak bırakıldı)

  -- Satınalma (3 PR, 2 PO)
  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-H001',v_admin,CURRENT_DATE+30,'Tibbi oksijen depolama tanki','Draft',now()-interval '27 day')
  RETURNING id INTO pr1;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit,note) VALUES
    (pr1,'Sivi O2 Depolama Tanki 15000 lt','HTM 02-01 / EN 13458',1.000,'adet','Hastane ana O2 rezervuari'),
    (pr1,'O2 Buharlas tiric i Unitesi','HTM 02-01',2.000,'adet','2 adet N+1 yedekli');
  UPDATE purchase_requests SET status='Approved', submitted_at=now()-interval '25 day',
    decided_by=v_admin, decided_at=now()-interval '23 day' WHERE id=pr1;

  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-H002',v_admin,CURRENT_DATE+45,'Jenerator grubu temin talebi','Draft',now()-interval '22 day')
  RETURNING id INTO pr2;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit) VALUES
    (pr2,'Jenerator 1000 kVA Dogalgaz','ISO 8528 / CE',3.000,'adet'),
    (pr2,'Otomatik Sebeke Degistirme Panosu ATS','IEC 60947-6-1',3.000,'adet');
  UPDATE purchase_requests SET status='Approved', submitted_at=now()-interval '20 day',
    decided_by=v_admin, decided_at=now()-interval '18 day' WHERE id=pr2;

  INSERT INTO purchase_requests (project_id,pr_no,requested_by,needed_by_date,note,status,created_at)
  VALUES (v_proj,'PR-H003',v_admin,CURRENT_DATE+60,'Ameliyathane steril hava santrali','Draft',now()-interval '5 day')
  RETURNING id INTO pr3;
  INSERT INTO purchase_request_items (pr_id,material_name,spec,qty,unit,note) VALUES
    (pr3,'Ameliyathane AHU Steril H14 HEPA','EN 1886 / VDI 6022',12.000,'adet','12 ameliyathane icin ayri ayri'),
    (pr3,'HEPA H14 Filtre Grubu','EN 1822',24.000,'adet','Her AHU icin 2 adet');
  UPDATE purchase_requests SET status='Submitted', submitted_at=now()-interval '3 day' WHERE id=pr3;

  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,expected_date,note,created_by,created_at)
  VALUES (v_proj,pr1,'PO-H001','Linde Gas Türkiye A.Ş.',2850000,'TRY','Ordered',
    CURRENT_DATE+28,'Sıvı O2 tankı ve buharlaştırıcı siparişi verildi.',v_admin,now()-interval '20 day')
  RETURNING id INTO po1;
  INSERT INTO purchase_orders (project_id,pr_id,po_no,supplier_name,amount,currency,status,expected_date,note,created_by,created_at)
  VALUES (v_proj,pr2,'PO-H002','Caterpillar Elektrik Sistemleri A.Ş.',9600000,'TRY','PartiallyDelivered',
    CURRENT_DATE+40,'3 adet 1000 kVA jeneratör + ATS. 1. jeneratör teslim alındı.',v_admin,now()-interval '15 day')
  RETURNING id INTO po2;

  RAISE NOTICE 'DEMO-04 verisi oluşturuldu (proje id: %)', v_proj;
END $$;
