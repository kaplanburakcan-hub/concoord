-- ============================================================================
-- İPKS — DEMO / TEST VERİSİ
-- ============================================================================
-- Amaç: boş ekranlarla gizlenen frontend/backend sorunlarını ortaya çıkarmak ve
-- EVM · S-eğrisi · portföy · İSG · satınalma akışlarını gerçekçi hacimde görmek.
--
-- Üretilenler (DEMO-01 projesi):
--   · 8 aylık takvimi olan aktif bir konut projesi (2025-12 → 2026-11)
--   · 4 taşeron + sözleşmeleri + birim fiyat pozları
--   · 6 dönem KESİNLEŞMİŞ hakediş (farklı finalized_at aylarıyla → S-eğrisi çizer)
--     + 1 onay bekleyen + 1 taslak
--   · avans mahsubu / teminat / İSG ceza kesintileri
--   · 6 milestone (tamamlanan, devam eden, geciken)
--   · PV planı (aylık planlanan %) → EVM "manual" plan kaynağı
--   · 9 İSG bulgusu (kritik/majör/minör/gözlem, açık-kapalı-termini geçmiş)
--   · 8 malzeme onayı (tüm durumlar)
--   · 12 görev (Kanban kolonlarına dağılmış)
--
-- GÜVENLİ: yalnızca kendi eklediği DEMO-01 projesini ve ona bağlı satırları
-- oluşturur; mevcut verilere DOKUNMAZ. Tekrar çalıştırılırsa önce eski DEMO-01
-- verisini temizler (idempotent).
--
-- Çalıştırma (proje kökünde):
--   docker compose -f deploy/docker-compose.yml --env-file .env exec -T postgres \
--     psql -U ipks -d ipks < deploy/seed/demo-data.sql
-- ============================================================================

DO $$
DECLARE
  v_admin   uuid;
  v_role_pm uuid;
  v_proj    uuid;
  -- taşeronlar
  s_kaba uuid; s_ince uuid; s_mek uuid; s_elek uuid;
  -- sözleşmeler
  c_kaba uuid; c_ince uuid; c_mek uuid; c_elek uuid;
  -- pozlar
  w_kalip uuid; w_demir uuid; w_beton uuid; w_duvar uuid; w_siva uuid;
  w_tesisat uuid; w_kablo uuid;
  pp        uuid;
  v_month   date;
  i         int;
  -- hakediş dönem tutarları (kümülatif brüt, TRY)
  gross_cum_arr numeric[] := ARRAY[4200000, 9800000, 16400000, 23100000, 29800000, 35600000];
  prev_cum  numeric := 0;
  this_amt  numeric;
  ded_adv   numeric;
  ded_ret   numeric;
  ded_ohs   numeric;
  ded_wht   numeric;
  ded_meal  numeric;
  ded_util  numeric;
  v_vat       numeric;
  v_vat_wh    numeric;
  v_vat_coll  numeric;
  v_payable   numeric;
  v_ded_total numeric;
  v_cost_red  numeric;
  v_actual    numeric;
  j         int;
  net_amt   numeric;
BEGIN
  -- --- Bağlam: bootstrap admin ve bir rol ---
  SELECT id INTO v_admin FROM users
   WHERE deleted_at IS NULL ORDER BY created_at LIMIT 1;
  IF v_admin IS NULL THEN
    RAISE EXCEPTION 'Kullanıcı bulunamadı — önce "make seed" çalıştırın.';
  END IF;

  SELECT id INTO v_role_pm FROM roles WHERE name = 'ProjectManager' LIMIT 1;
  IF v_role_pm IS NULL THEN
    SELECT id INTO v_role_pm FROM roles ORDER BY created_at LIMIT 1;
  END IF;

  -- --- Temizlik: önceki DEMO-01 çalıştırması ---
  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-01';
  IF v_proj IS NOT NULL THEN
    -- Önceki DEMO-01 ARŞİVLENİR, silinmez.
    --
    -- Sert silme bilinçli olarak tercih EDİLMEDİ: bir projeye 20'den fazla tablo
    -- bağlıdır (bildirimler, dokümanlar, satınalma, İSG denetimleri, raporlar…)
    -- ve bunların da alt tabloları vardır; hepsini doğru sırayla silmek kırılgandır.
    -- Daha önemlisi, sistemin temel ilkesi "hiçbir veri kaybolmaz"dır (Plan §3):
    -- kesinleşmiş hakedişler ve denetim izi kalıcıdır. Bu yüzden eski demo projesi
    -- zaman damgalı bir kodla soft-delete edilir; listelerde görünmez, geçmişi
    -- bozulmaz, yeni DEMO-01 temiz şekilde kurulur.
    UPDATE projects
       SET code = 'DEMO-01-ARSIV-' || to_char(now(), 'YYYYMMDDHH24MISS'),
           name = name || ' (arşiv)',
           status = 'Archived',
           deleted_at = now()
     WHERE id = v_proj;
    RAISE NOTICE 'Önceki DEMO-01 arşivlendi (id: %)', v_proj;
  END IF;

  -- ==========================================================
  -- 1) PROJE
  -- ==========================================================
  INSERT INTO projects (code, name, location, client_name, budget_total,
                        currency, start_date, end_date, status)
  VALUES ('DEMO-01', 'Bahçeşehir 480 Konut ve Sosyal Tesis',
          'İstanbul / Başakşehir', 'Emlak Konut GYO',
          62000000, 'TRY', DATE '2025-12-01', DATE '2026-11-30', 'Active')
  RETURNING id INTO v_proj;

  INSERT INTO project_members (project_id, user_id, role_id)
  VALUES (v_proj, v_admin, v_role_pm);

  -- ==========================================================
  -- 2) TAŞERONLAR
  -- ==========================================================
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Anadolu Kaba Yapı A.Ş.','1234567890','Mehmet Yılmaz','0532 111 2233','info@anadolukaba.com','Kaba Yapı')
  RETURNING id INTO s_kaba;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Marmara İnce İşler Ltd.','2345678901','Ayşe Demir','0533 222 3344','iletisim@marmarainc.com','İnce İşler')
  RETURNING id INTO s_ince;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Ege Mekanik Tesisat A.Ş.','3456789012','Can Öztürk','0534 333 4455','proje@egemekanik.com','Mekanik')
  RETURNING id INTO s_mek;
  INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
  VALUES (v_proj,'Volt Elektrik Taahhüt Ltd.','4567890123','Zeynep Kaya','0535 444 5566','info@voltelektrik.com','Elektrik')
  RETURNING id INTO s_elek;

  -- ==========================================================
  -- 3) SÖZLEŞMELER
  -- ==========================================================
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
                         advance_amount, retention_pct, advance_rate_pct, sign_date,
                         start_date, end_date, is_multi_year, withholding_pct)
  VALUES (v_proj,s_kaba,'SZL-2025-001','Sub',28000000,4200000,5.00,20.00,DATE '2025-11-20',
          DATE '2025-12-01', DATE '2026-11-30', true, 5.00)
  RETURNING id INTO c_kaba;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
                         advance_amount, retention_pct, advance_rate_pct, sign_date)
  VALUES (v_proj,s_ince,'SZL-2025-002','Sub',14500000,2000000,5.00,20.00,DATE '2025-12-05')
  RETURNING id INTO c_ince;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
                         advance_amount, retention_pct, advance_rate_pct, sign_date)
  VALUES (v_proj,s_mek,'SZL-2026-003','Sub',9800000,1500000,3.00,15.00,DATE '2026-01-15')
  RETURNING id INTO c_mek;
  INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, amount,
                         advance_amount, retention_pct, advance_rate_pct, sign_date)
  VALUES (v_proj,s_elek,'SZL-2026-004','Sub',7300000,1000000,3.00,15.00,DATE '2026-01-20')
  RETURNING id INTO c_elek;

  -- ==========================================================
  -- 4) BİRİM FİYAT POZLARI
  -- ==========================================================
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaba,c_kaba,'Y.16.050/03','Kalıp yapılması (plywood)','m2',48000,420.00) RETURNING id INTO w_kalip;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaba,c_kaba,'Y.15.140/02','Nervürlü çelik hasır donatı','ton',1850,28500.00) RETURNING id INTO w_demir;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_kaba,c_kaba,'Y.16.058/04','C30/37 hazır beton dökümü','m3',12400,3150.00) RETURNING id INTO w_beton;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_ince,c_ince,'Y.18.185/01','Gazbeton duvar örülmesi','m2',36000,285.00) RETURNING id INTO w_duvar;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_ince,c_ince,'Y.27.531/01','İç cephe alçı sıva','m2',52000,195.00) RETURNING id INTO w_siva;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_mek,c_mek,'Y.071.100','Pis su tesisatı PPRC boru','m',18500,340.00) RETURNING id INTO w_tesisat;
  INSERT INTO work_items (project_id,subcontractor_id,contract_id,poz_no,description,unit,contract_qty,unit_price)
  VALUES (v_proj,s_elek,c_elek,'Y.742.501','NYY kablo çekilmesi (3x2.5)','m',42000,128.00) RETURNING id INTO w_kablo;

  -- ==========================================================
  -- 5) HAKEDİŞLER — 6 dönem KESİNLEŞMİŞ (S-eğrisi için farklı aylar)
  -- ==========================================================
  FOR i IN 1..6 LOOP
    v_month  := (DATE '2026-01-01' + ((i - 1) || ' month')::interval)::date;
    this_amt := gross_cum_arr[i] - prev_cum;

    -- Finansman kesintileri (KDV'siz, dönem brütü üzerinden)
    ded_adv  := round(this_amt * 0.20, 2);                    -- avans mahsubu %20
    ded_ret  := round(this_amt * 0.05, 2);                    -- teminat %5
    ded_wht  := round(this_amt * 0.05, 2);                    -- stopaj %5 (yıllara sari)
    -- Mal/hizmet ve ceza kesintileri (KDV DAHİL tutarlar)
    ded_ohs  := CASE WHEN i IN (3,5) THEN 45000 ELSE 0 END;   -- İSG cezası, KDV'siz
    ded_meal := round(this_amt * 0.012, 2);                   -- yemek bedeli, KDV %10 dahil
    ded_util := round(this_amt * 0.008, 2);                   -- elektrik/su, KDV %20 dahil

    -- KDV: dönem brütü üzerinden, tevkifat 4/10
    v_vat       := round(this_amt * 0.20, 2);
    v_vat_wh    := round(v_vat * 0.40, 2);
    v_vat_coll  := v_vat - v_vat_wh;
    v_payable   := this_amt + v_vat_coll;
    v_ded_total := ded_adv + ded_ret + ded_wht + ded_ohs + ded_meal + ded_util;
    net_amt     := v_payable - v_ded_total;

    -- EVM maliyeti: brüt − maliyeti azaltan kesintilerin KDV HARİÇ tutarı
    v_cost_red  := ded_ohs + round(ded_meal / 1.10, 2) + round(ded_util / 1.20, 2);
    v_actual    := this_amt - v_cost_red;

    INSERT INTO progress_payments (
      project_id, subcontractor_id, period_no, period_start, period_end, status,
      gross_cum, gross_prev, gross_this, total_deductions, vat_pct, net_payable,
      vat_amount, vat_withheld, vat_collected, payable_gross, actual_cost,
      vat_withholding_ratio, withholding_applied, current_step_no,
      submitted_at, site_approved_at, finalized_at, finalized_by, created_by, created_at)
    VALUES (
      v_proj, s_kaba, i,
      date_trunc('month', v_month)::date,
      (date_trunc('month', v_month) + interval '1 month - 1 day')::date,
      'SiteApproved',
      gross_cum_arr[i], prev_cum, this_amt, v_ded_total, 20.00, net_amt,
      v_vat, v_vat_wh, v_vat_coll, v_payable, v_actual,
      0.40, true, 9,
      (v_month + interval '25 day'), (v_month + interval '27 day'),
      (v_month + interval '28 day'),  -- EV/AC bu aya düşer
      v_admin, v_admin, (v_month + interval '20 day'))
    RETURNING id INTO pp;

    INSERT INTO payment_deductions
      (progress_payment_id, type, description, rate_pct, amount, nature, reduces_cost,
       group_code, catalog_code, vat_pct, net_amount)
    VALUES
      (pp,'AdvanceOffset','Avans mahsubu',20.0000,ded_adv,'Offset',false,
       'Advance','ADV_CONTRACT',0,ded_adv),
      (pp,'Retention','Teminat kesintisi',5.0000,ded_ret,'Temporary',false,
       'Retention','RET_PERFORMANCE',0,ded_ret),
      (pp,'Withholding','Stopaj (yıllara sari)',5.0000,ded_wht,'Permanent',false,
       'Tax','WHT_INCOME',0,ded_wht),
      (pp,'Other','Öğle yemeği bedeli',NULL,ded_meal,'Permanent',true,
       'GoodsService','GS_LUNCH',10,round(ded_meal/1.10,2)),
      (pp,'Other','Elektrik ve su bedeli',NULL,ded_util,'Permanent',true,
       'GoodsService','GS_ELECTRICITY',20,round(ded_util/1.20,2));

    IF ded_ohs > 0 THEN
      INSERT INTO payment_deductions
        (progress_payment_id, type, source_entity, description, amount, nature, reduces_cost,
         group_code, catalog_code, vat_pct, net_amount)
      VALUES (pp,'OHSPenalty','ohs_penalties','İSG ceza tutanağı kesintisi',ded_ohs,
              'Permanent',true,'Penalty','PEN_OHS',0,ded_ohs);
    END IF;

    INSERT INTO progress_payment_items
      (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
    VALUES
      (pp, w_kalip, round((prev_cum/420.0)*0.30,3), round((this_amt/420.0)*0.30,3),
           round((gross_cum_arr[i]/420.0)*0.30,3), round(gross_cum_arr[i]*0.30,2), round(this_amt*0.30,2)),
      (pp, w_demir, round((prev_cum/28500.0)*0.35,3), round((this_amt/28500.0)*0.35,3),
           round((gross_cum_arr[i]/28500.0)*0.35,3), round(gross_cum_arr[i]*0.35,2), round(this_amt*0.35,2)),
      (pp, w_beton, round((prev_cum/3150.0)*0.35,3), round((this_amt/3150.0)*0.35,3),
           round((gross_cum_arr[i]/3150.0)*0.35,3), round(gross_cum_arr[i]*0.35,2), round(this_amt*0.35,2));

    -- 9 adımlı onay zincirinin tamamı geçmiş olarak yazılır.
    FOR j IN 1..9 LOOP
      INSERT INTO payment_approvals (progress_payment_id, step_no, step_code, decision, actor_id, created_at)
      SELECT pp, s.step_no, s.code, 'Approved', v_admin, (v_month + interval '27 day') + (j || ' hour')::interval
      FROM payment_approval_steps s WHERE s.project_id IS NULL AND s.step_no = j;
    END LOOP;

    -- Alt kayıtlar yazıldı; şimdi kesinleştir (kilit bundan sonra devreye girer).
    UPDATE progress_payments SET status = 'Finalized' WHERE id = pp;

    prev_cum := gross_cum_arr[i];
  END LOOP;

  -- Onay bekleyen (SiteApproved) — 7. dönem
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,
    submitted_at,site_approved_at,created_by,created_at)
  VALUES (v_proj,s_kaba,7,DATE '2026-07-01',DATE '2026-07-31','SiteApproved',
    41200000,35600000,5600000,1400000,20.00,4200000,
    now() - interval '6 day', now() - interval '4 day', v_admin, now() - interval '8 day');

  -- Taslak — ince işler taşeronu
  INSERT INTO progress_payments (project_id,subcontractor_id,period_no,period_start,period_end,status,
    gross_cum,gross_prev,gross_this,total_deductions,vat_pct,net_payable,created_by,created_at)
  VALUES (v_proj,s_ince,1,DATE '2026-07-01',DATE '2026-07-31','Draft',
    3850000,0,3850000,962500,20.00,2887500,v_admin, now() - interval '2 day');

  -- ==========================================================
  -- 6) MİLESTONE'LAR
  -- ==========================================================
  INSERT INTO milestones (project_id,name,planned_date,actual_date,weight_pct,status,sort_order) VALUES
    (v_proj,'Hafriyat ve zemin iyileştirme',DATE '2025-12-31',DATE '2025-12-28',8.00,'Completed',1),
    (v_proj,'Temel ve bodrum betonarme',    DATE '2026-02-28',DATE '2026-03-05',18.00,'Completed',2),
    (v_proj,'Kaba yapı — normal katlar',    DATE '2026-06-30',NULL,32.00,'InProgress',3),
    (v_proj,'Duvar ve iç sıva imalatları',  DATE '2026-08-31',NULL,20.00,'Planned',4),
    (v_proj,'Mekanik ve elektrik tesisat',  DATE '2026-06-15',NULL,14.00,'Delayed',5),
    (v_proj,'İnce işler ve geçici kabul',   DATE '2026-11-30',NULL,8.00,'Planned',6);

  -- ==========================================================
  -- 7) PV PLANI → EVM plan kaynağı "manual"
  -- DİKKAT: planned_pct O AYA AİT ARTIŞ yüzdesidir (kümülatif DEĞİL).
  -- Sistem bunları aydan aya toplayarak kümülatif PV eğrisini kurar (Σ = 100).
  -- ==========================================================
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

  -- ==========================================================
  -- 8) İSG BULGULARI
  -- ==========================================================
  INSERT INTO ohs_findings (project_id,subcontractor_id,severity,description,location,
                            gps_lat,gps_lng,due_date,status,reported_by,created_at) VALUES
    (v_proj,s_kaba,'Critical','B blok 6. kat kenar korkuluğu sökülmüş, düşme riski.','B Blok / 6. kat',
      41.0951,28.8012,CURRENT_DATE - 4,'Open',v_admin, now() - interval '9 day'),
    (v_proj,s_kaba,'Critical','Kule vinç günlük kontrol formu 3 gündür imzalanmamış.','Şantiye sahası',
      41.0949,28.8009,CURRENT_DATE - 1,'InProgress',v_admin, now() - interval '6 day'),
    (v_proj,s_ince,'Major','İskele platformunda eksik dilme, 2 noktada boşluk var.','A Blok cephe',
      41.0953,28.8015,CURRENT_DATE + 3,'Open',v_admin, now() - interval '5 day'),
    (v_proj,s_mek,'Major','Kaynak işlerinde yangın söndürücü bulundurulmamış.','Mekanik oda',
      41.0947,28.8006,CURRENT_DATE + 5,'Open',v_admin, now() - interval '4 day'),
    (v_proj,s_elek,'Major','Geçici elektrik panosunda topraklama eksik.','Şantiye girişi',
      41.0944,28.8003,CURRENT_DATE - 2,'Open',v_admin, now() - interval '7 day'),
    (v_proj,s_kaba,'Minor','İki işçide baret takılı değil (uyarı yapıldı).','C Blok / 2. kat',
      41.0955,28.8018,CURRENT_DATE + 7,'Open',v_admin, now() - interval '3 day'),
    (v_proj,s_ince,'Minor','Malzeme istifi geçiş yolunu daraltıyor.','Depo alanı',
      41.0942,28.8000,CURRENT_DATE + 9,'InProgress',v_admin, now() - interval '2 day'),
    (v_proj,s_kaba,'Observation','İSG panosu güncel değil, eski duyuru asılı.','Şantiye şefliği',
      41.0940,28.7998,CURRENT_DATE + 14,'Open',v_admin, now() - interval '1 day'),
    (v_proj,s_elek,'Minor','Uzatma kablosu zeminde, koruma kanalı yok.','A Blok / zemin kat',
      41.0951,28.8010,CURRENT_DATE - 10,'Open',v_admin, now() - interval '20 day');

  -- Kapanmış bulgu: trigger kapalı kaydın DEĞİŞTİRİLMESİNİ engeller, bu yüzden
  -- kayıt Open olarak açılır ve tek UPDATE ile kapatılır (Open→Closed serbest).
  UPDATE ohs_findings
     SET status = 'Closed', closed_by = v_admin, closed_at = now() - interval '12 day',
         close_note = 'Kablo kanalı çekildi, saha kontrolü yapıldı.'
   WHERE project_id = v_proj
     AND description = 'Uzatma kablosu zeminde, koruma kanalı yok.';

  -- ==========================================================
  -- 9) MALZEME ONAYLARI (MAR)
  -- ==========================================================
  -- NOT: chk_mar_decision_note kısıtı gereği Submitted/UnderReview dışındaki
  -- durumlarda decision_note INSERT anında dolu olmak zorundadır.
  INSERT INTO material_approvals (project_id,subcontractor_id,mar_no,material_name,spec_ref,
                                  manufacturer,status,decision_note,decided_by,decided_at,
                                  created_by,created_at) VALUES
    (v_proj,s_kaba,'MAR-001','C30/37 Hazır Beton','TS EN 206-1','Akçansa Beton','Approved',
      'Numune ve deney raporları şartnameye uygun bulundu.',v_admin,now()-interval '65 day',v_admin,now()-interval '70 day'),
    (v_proj,s_kaba,'MAR-002','B500C Nervürlü İnşaat Demiri','TS 708','İçdaş Çelik','Approved',
      'Çekme deneyi sonuçları uygun; onaylandı.',v_admin,now()-interval '60 day',v_admin,now()-interval '65 day'),
    (v_proj,s_ince,'MAR-003','Gazbeton Blok G2/400','TS EN 771-4','Ytong','Approved',
      'Ürün belgeleri ve CE beyanı tam.',v_admin,now()-interval '35 day',v_admin,now()-interval '40 day'),
    (v_proj,s_ince,'MAR-004','Alçı Sıva (makine sıvası)','TS EN 13279-1','Knauf','ConditionallyApproved',
      'Onaylandı; ilk uygulamada mahal numunesi alınacak.',v_admin,now()-interval '20 day',v_admin,now()-interval '25 day'),
    (v_proj,s_mek,'MAR-005','PPRC Boru ve Ek Parçaları','TS EN ISO 15874','Firat Plastik','UnderReview',
      NULL,NULL,NULL,v_admin,now()-interval '12 day'),
    (v_proj,s_elek,'MAR-006','NYY Kablo 3x2.5 mm2','TS IEC 60502-1','Öznur Kablo','UnderReview',
      NULL,NULL,NULL,v_admin,now()-interval '9 day'),
    (v_proj,s_elek,'MAR-007','Sıva Altı Anahtar-Priz Serisi','TS EN 60669','Viko','Submitted',
      NULL,NULL,NULL,v_admin,now()-interval '4 day'),
    (v_proj,s_mek,'MAR-008','Radyatör Panel Tip 22','TS EN 442','DemirDöküm','Rejected',
      'Isıl güç değerleri projede öngörülenin altında, revize teklif bekleniyor.',v_admin,now()-interval '25 day',v_admin,now()-interval '30 day');

  -- ==========================================================
  -- 10) GÖREVLER (Kanban)
  -- ==========================================================
  INSERT INTO tasks (project_id,title,description,status,priority,due_date,created_by,kanban_order,created_at) VALUES
    (v_proj,'B blok kenar korkuluklarının yenilenmesi','Kritik İSG bulgusu kapatılacak.','InProgress','Urgent',CURRENT_DATE+2,v_admin,1,now()-interval '9 day'),
    (v_proj,'Kule vinç periyodik kontrol raporu','Yetkili firma raporu dosyaya eklenecek.','Todo','Urgent',CURRENT_DATE+1,v_admin,1,now()-interval '6 day'),
    (v_proj,'7. dönem hakediş kesinleşmesi','Saha onayı tamam, PY kesinleştirmesi bekleniyor.','Review','High',CURRENT_DATE+3,v_admin,1,now()-interval '4 day'),
    (v_proj,'Mekanik tesisat gecikme aksiyon planı','Milestone gecikmesi için telafi programı.','InProgress','High',CURRENT_DATE+6,v_admin,2,now()-interval '11 day'),
    (v_proj,'PPRC boru MAR incelemesi','Teknik föy ve deney raporu kontrolü.','InProgress','Normal',CURRENT_DATE+4,v_admin,3,now()-interval '8 day'),
    (v_proj,'C blok kalıp iskele revizyonu','Kat yüksekliği değişikliği sonrası.','Todo','Normal',CURRENT_DATE+10,v_admin,2,now()-interval '5 day'),
    (v_proj,'Şantiye İSG eğitimi (aylık)','Yeni giren 14 personel için.','Todo','Normal',CURRENT_DATE+8,v_admin,3,now()-interval '3 day'),
    (v_proj,'Asansör tedarikçi teklif değerlendirmesi','3 teklif karşılaştırma tablosu.','Backlog','Normal',CURRENT_DATE+20,v_admin,1,now()-interval '15 day'),
    (v_proj,'Peyzaj projesi revizyon talebi','İdare görüşü bekleniyor.','Backlog','Low',CURRENT_DATE+30,v_admin,2,now()-interval '18 day'),
    (v_proj,'Temel betonarme as-built çizimleri','Tamamlandı, arşive alındı.','Done','Normal',CURRENT_DATE-12,v_admin,1,now()-interval '45 day'),
    (v_proj,'Gazbeton MAR onayı','Onaylandı ve saha bilgilendirildi.','Done','Normal',CURRENT_DATE-20,v_admin,2,now()-interval '40 day'),
    (v_proj,'Hafriyat kabul tutanağı','İdare ile birlikte imzalandı.','Done','Low',CURRENT_DATE-60,v_admin,3,now()-interval '70 day');

  RAISE NOTICE 'DEMO-01 verisi oluşturuldu (proje id: %)', v_proj;
END $$;
