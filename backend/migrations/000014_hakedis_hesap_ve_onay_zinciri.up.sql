-- ============================================================================
-- Faz 11 / 000014 — Gerçek hakediş hesap sırası ve çok adımlı onay zinciri
-- ============================================================================
-- Gerçek bir taşeron hakedişi (saha örneği) iki konuda modelden ayrılıyordu:
--
-- 1) HESAP SIRASI. Kesintiler net tutardan değil, KDV DAHİL ÖDENEBİLİR
--    TOPLAMDAN düşülür:
--
--        C  = dönem brütü (KDV hariç)
--        KDV = C × oran            → tevkifat kadarı vergi dairesine
--        Ödenebilir toplam = C + tahsil edilen KDV
--        Yükleniciye ödenecek = Ödenebilir toplam − Kesintiler (KDV dahil)
--
--    Sebebi ekonomiktir: taşeron işin tamamını faturalar (KDV tam brüt
--    üzerinden); ana yüklenici ona ayrıca yemek/elektrik/malzeme satar. İki
--    fatura mahsuplaşır. Bu yüzden HER KESİNTİ KALEMİNİN KENDİ KDV ORANI vardır
--    (ceza %0, yemek %8/%10, malzeme-hizmet %20) ve tutarlar KDV dahildir.
--
--    EVM Gerçekleşen Maliyet (AC) ise KDV HARİÇ çalışır: KDV devreden bir
--    kalemdir, maliyet değildir.
--
-- 2) ONAY ZİNCİRİ. Saha uygulamasında hakediş 9 kademeden geçer (teknik ofis,
--    kalite, İSG, şantiye müdürü, merkez ofis, genel müdür, muhasebe, finans).
--    Zincir SABİT KODLANMAZ — veri olarak tutulur ki her kurum kendi akışını
--    tanımlayabilsin (Plan §3: "yetki koda gömülmez, veriye yazılır").
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1) Hakediş: KDV ve maliyet alanları
-- ---------------------------------------------------------------------------
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS vat_amount     numeric(18,2) NOT NULL DEFAULT 0, -- hesaplanan KDV
    ADD COLUMN IF NOT EXISTS vat_withheld   numeric(18,2) NOT NULL DEFAULT 0, -- tevkif edilen (vergi dairesine)
    ADD COLUMN IF NOT EXISTS vat_collected  numeric(18,2) NOT NULL DEFAULT 0, -- tahsil edilen (ödemeye eklenen)
    ADD COLUMN IF NOT EXISTS payable_gross  numeric(18,2) NOT NULL DEFAULT 0, -- C + tahsil edilen KDV
    ADD COLUMN IF NOT EXISTS actual_cost    numeric(18,2) NOT NULL DEFAULT 0; -- EVM AC (KDV hariç)

COMMENT ON COLUMN progress_payments.net_payable IS
    'Yükleniciye ödenecek nihai tutar = payable_gross − total_deductions (KDV dahil kesintiler)';
COMMENT ON COLUMN progress_payments.actual_cost IS
    'EVM Gerçekleşen Maliyet: gross_this − maliyeti azaltan kesintilerin KDV hariç tutarı';

-- ---------------------------------------------------------------------------
-- 2) Kesinti: kendi KDV oranı
-- ---------------------------------------------------------------------------
ALTER TABLE payment_deductions
    ADD COLUMN IF NOT EXISTS vat_pct    numeric(5,2)  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS net_amount numeric(18,2) NOT NULL DEFAULT 0; -- amount'un KDV hariç kısmı

COMMENT ON COLUMN payment_deductions.amount IS
    'Ödenebilir toplamdan düşülen tutar (mal/hizmet kesintilerinde KDV DAHİL)';
COMMENT ON COLUMN payment_deductions.net_amount IS
    'amount / (1 + vat_pct/100) — maliyet hesabında (AC) bu değer kullanılır';

-- Mevcut kayıtlar: KDV oranı bilinmiyor, tutarlar KDV hariç kabul edilir.
-- Kilit trigger'ı şema geçişi boyunca devre dışı (bkz. 000012 açıklaması).
ALTER TABLE payment_deductions DISABLE TRIGGER trg_pd_lock;
UPDATE payment_deductions SET net_amount = amount WHERE net_amount = 0;
ALTER TABLE payment_deductions ENABLE TRIGGER trg_pd_lock;

-- ---------------------------------------------------------------------------
-- 3) Katalog: bütçe kodu ve varsayılan KDV oranı
-- ---------------------------------------------------------------------------
ALTER TABLE deduction_catalog
    ADD COLUMN IF NOT EXISTS budget_code     text NULL,
    ADD COLUMN IF NOT EXISTS default_vat_pct numeric(5,2) NOT NULL DEFAULT 0;

COMMENT ON COLUMN deduction_catalog.budget_code IS
    'Muhasebe/bütçe hesap kodu (BKA) — kesinti satırı bu koda işlenir';

-- Mal/hizmet kesintileri KDV'lidir; ceza ve finansman kalemleri KDV'siz.
UPDATE deduction_catalog SET default_vat_pct = 20 WHERE group_code = 'GoodsService';
UPDATE deduction_catalog SET default_vat_pct = 10
 WHERE code IN ('GS_MEAL','GS_LODGING');
UPDATE deduction_catalog SET default_vat_pct = 0
 WHERE group_code IN ('Tax','Advance','Retention','Penalty');

-- Damga vergisi muafiyeti olan projeler için oran 0 seçilebilmeli.
UPDATE deduction_catalog SET note = COALESCE(note,'') ||
    ' Muafiyet varsa oran 0 girilerek kesinti yapılmaz.'
 WHERE code = 'STAMP_DUTY';

-- Saha örneğinden gelen ek kalemler.
INSERT INTO deduction_catalog
  (group_code, code, label, deduction_type, nature, reduces_cost, default_vat_pct, budget_code, sort_order)
VALUES
('Tax','WHT_ADVANCE','Avans ödemesine istinaden tahakkuk eden stopaj','Withholding','Permanent',false,0,NULL,15),
('GoodsService','GS_OSGB','OSGB (ortak sağlık güvenlik birimi) bedeli','Other','Permanent',true,20,'14050250',15),
('GoodsService','GS_BREAKFAST','Sabah kahvaltısı bedeli','Other','Permanent',true,10,'14020410',11),
('GoodsService','GS_LUNCH','Öğle yemeği bedeli','Other','Permanent',true,10,'14020410',12),
('GoodsService','GS_DINNER','Akşam yemeği bedeli','Other','Permanent',true,10,'14020410',13),
('GoodsService','GS_RATION','Kumanya bedeli','Other','Permanent',true,10,'14020410',14),
('GoodsService','GS_LUNCH_WC','Öğle yemeği bedeli (beyaz yaka)','Other','Permanent',true,10,'14020410',15),
('GoodsService','GS_DINNER_WC','Akşam yemeği bedeli (beyaz yaka)','Other','Permanent',true,10,'14020410',16),
('GoodsService','GS_OFFICE_USE','Şantiye ofis kullanım bedeli','Other','Permanent',true,20,'14120211',115),
('GoodsService','GS_CONTAINER','Şantiye konteyner bedeli','Other','Permanent',true,20,'14120211',116),
('GoodsService','GS_DORM','Koğuş kullanım bedeli','Other','Permanent',true,20,'14120240',117),
('GoodsService','GS_MACHINE_LOG','İş makinesi tutanağı','Other','Permanent',true,20,'13040210',111),
('GoodsService','GS_MOBILIZATION','Mobilizasyon / demobilizasyon','Other','Permanent',true,20,'14070110',185),
('GoodsService','GS_SITE_PANEL','Geçici saha panoları','Other','Permanent',true,20,'14070430',186),
('GoodsService','GS_LIGHT_CABLE','Aydınlatma kabloları','Other','Permanent',true,20,'14070410',187),
('GoodsService','GS_POWER_CABLE','Güç kabloları','Other','Permanent',true,20,'14070411',188),
('GoodsService','GS_DRAIN_MAT','Atıksu / drenaj malzemeleri','Other','Permanent',true,20,'14070310',189),
('GoodsService','GS_WATER_MAT','Su temini malzemeleri','Other','Permanent',true,20,'14070320',190),
('GoodsService','GS_INFIRMARY','Revir ve sağlık giderleri','Other','Permanent',true,20,'14050260',191),
('GoodsService','GS_PHYS_PROTECT','Fiziksel koruyucu önlemler','Other','Permanent',true,20,'14050230',192),
('GoodsService','GS_SAFETY_SIGN','İş güvenliği uyarıcı levhaları','Other','Permanent',true,20,'14050240',193),
('GoodsService','GS_SITE_SHARE','Şantiye katılım bedeli','Other','Permanent',true,20,'14020411',194),
('Penalty','PEN_CONTRACT','Sözleşmeden doğan ceza kesintisi','Other','Permanent',true,0,NULL,15)
ON CONFLICT (code) DO NOTHING;

-- Mevcut katalog kalemlerine bütçe kodu ata (saha örneğiyle eşleşenler).
UPDATE deduction_catalog SET budget_code='14050250' WHERE code='PEN_OHS'        AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='14020411' WHERE code='GS_LODGING'     AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='13010130' WHERE code='GS_CLEANING'    AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='14030310' WHERE code='GS_ELECTRICITY' AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='14030320' WHERE code='GS_WATER'       AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='14030821' WHERE code='GS_FUEL'        AND budget_code IS NULL;
UPDATE deduction_catalog SET budget_code='14020416' WHERE code='GS_TRANSPORT'   AND budget_code IS NULL;

-- ---------------------------------------------------------------------------
-- 4) ONAY ZİNCİRİ (yapılandırılabilir)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payment_approval_steps (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NULL REFERENCES projects (id) ON DELETE CASCADE, -- NULL = global varsayılan zincir
    step_no     integer NOT NULL,
    code        text NOT NULL,
    label       text NOT NULL,
    permission  text NOT NULL,          -- bu adımı onaylayabilmek için gereken izin
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pas_step
    ON payment_approval_steps (COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), step_no);

DROP TRIGGER IF EXISTS trg_pas_updated_at ON payment_approval_steps;
CREATE TRIGGER trg_pas_updated_at BEFORE UPDATE ON payment_approval_steps
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Kim, hangi adımı, ne zaman onayladı (kayıt defteri — silinmez).
CREATE TABLE IF NOT EXISTS payment_approvals (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    progress_payment_id uuid NOT NULL REFERENCES progress_payments (id) ON DELETE RESTRICT,
    step_no             integer NOT NULL,
    step_code           text NOT NULL,
    decision            text NOT NULL CHECK (decision IN ('Approved','Rejected')),
    note                text NULL,
    actor_id            uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pa_payment ON payment_approvals (progress_payment_id, step_no);

-- Hakedişin zincirdeki konumu.
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS current_step_no integer NOT NULL DEFAULT 0;

COMMENT ON COLUMN progress_payments.current_step_no IS
    '0 = henüz onay sürecine girmedi; n = n. adım onaylandı, n+1 bekleniyor';

-- Durum kümesine çok adımlı akış için InApproval eklenir.
ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS progress_payments_status_check;
ALTER TABLE progress_payments
    ADD CONSTRAINT progress_payments_status_check
    CHECK (status IN ('Draft','Submitted','InApproval','SiteApproved','Finalized','Rejected'));

-- Varsayılan 9 adımlı zincir (saha uygulamasından). Kurumlar proje bazında
-- kendi zincirini tanımlayarak bunu geçersiz kılabilir.
INSERT INTO payment_approval_steps (project_id, step_no, code, label, permission) VALUES
(NULL, 1, 'TECH_OFFICE_SPECIALIST', 'Teknik Ofis Uzmanı',            'progress_payments.approve'),
(NULL, 2, 'QUALITY_CHIEF',          'Kalite Kontrol Şefi',           'progress_payments.approve'),
(NULL, 3, 'OHS_CHIEF',              'İSG-Ç Şefi',                    'progress_payments.approve'),
(NULL, 4, 'TECH_OFFICE_CHIEF',      'Teknik Ofis Şefi',              'progress_payments.approve'),
(NULL, 5, 'SITE_MANAGER',           'Şantiye Müdürü',                'progress_payments.approve'),
(NULL, 6, 'CENTRAL_TECH_OFFICE',    'Merkez Teknik Ofis',            'progress_payments.finalize'),
(NULL, 7, 'EXECUTIVE',              'Genel Müdür / Yardımcısı',      'progress_payments.finalize'),
(NULL, 8, 'ACCOUNTING',             'Muhasebe Kontrol',              'progress_payments.finalize'),
(NULL, 9, 'FINANCIAL_CONTROL',      'Finansal Kontrol',              'progress_payments.finalize')
ON CONFLICT DO NOTHING;
