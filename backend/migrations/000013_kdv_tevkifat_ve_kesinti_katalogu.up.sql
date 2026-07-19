-- ============================================================================
-- Faz 11 / 000013 — KDV tevkifatı, KDV istisnası, kesinti kataloğu,
--                   sözleşme süresi ve hakediş dönem kontrolleri
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1) KDV TEVKİFATI ve İSTİSNA (hakediş başlığı)
--
-- Tevkifat diğer kesintilerden YAPISAL OLARAK FARKLIDIR: brütten değil, KDV
-- TUTARI ÜZERİNDEN uygulanır. Yapım işlerinde oran 4/10'dur: hesaplanan KDV'nin
-- %40'ı taşerona ödenmez, alıcı (ana yüklenici) tarafından doğrudan vergi
-- dairesine yatırılır. Bu yüzden:
--     Ödenecek KDV = Hesaplanan KDV − Tevkif edilen KDV
--     Genel toplam = Net (KDV hariç) + Ödenecek KDV
-- Tevkifat İŞİN MALİYETİNİ (EVM AC) ETKİLEMEZ; yalnızca ödeme tutarını değiştirir.
--
-- KDV oranı mal/hizmet türüne göre %0, %10 veya %20 olabilir. Oran 0 ise
-- gerekçesi (istisna kodu) kayda geçer — hakediş kayıt defteri bütünlüğü için.
-- ---------------------------------------------------------------------------
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS vat_withholding_ratio numeric(6,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS vat_exemption_code    text NULL;

ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS chk_pp_vat_withholding;
ALTER TABLE progress_payments
    ADD CONSTRAINT chk_pp_vat_withholding
    CHECK (vat_withholding_ratio >= 0 AND vat_withholding_ratio <= 1);

COMMENT ON COLUMN progress_payments.vat_withholding_ratio IS
    'KDV tevkifat oranı (0=yok, 0.4=4/10 yapım işleri, 0.5=5/10, 0.2=2/10)';
COMMENT ON COLUMN progress_payments.vat_exemption_code IS
    'KDV %0 uygulandığında istisna gerekçesi (ör. 13/a yatırım teşvik, 11/1-a ihracat)';

-- ---------------------------------------------------------------------------
-- 2) SÖZLEŞME SÜRESİ
--
-- Hakediş döneminin sözleşme süresini aşıp aşmadığı kontrol edilebilsin diye
-- sözleşmeye işe başlama/bitiş tarihi eklenir. Süre uzatımı verildiğinde
-- revised_end_date doldurulur; kontroller uzatılmış tarihi esas alır.
-- ---------------------------------------------------------------------------
ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS start_date        date NULL,
    ADD COLUMN IF NOT EXISTS end_date          date NULL,
    ADD COLUMN IF NOT EXISTS revised_end_date  date NULL;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_dates;
ALTER TABLE contracts
    ADD CONSTRAINT chk_contracts_dates CHECK (
        (start_date IS NULL OR end_date IS NULL OR end_date >= start_date)
        AND (revised_end_date IS NULL OR start_date IS NULL OR revised_end_date >= start_date)
    );

COMMENT ON COLUMN contracts.revised_end_date IS
    'Süre uzatımı sonrası bitiş; hakediş dönem kontrolleri bu tarihi esas alır';

-- ---------------------------------------------------------------------------
-- 3) HAKEDİŞ DÖNEM KONTROLLERİ
--
-- Şimdiye dek yalnızca dönem NUMARASI tekildi; aynı taşeron için aynı TARİH
-- ARALIĞINA ikinci bir hakediş girilebiliyordu. Tarih aralığı çakışması
-- veritabanı düzeyinde engellenir (uygulama katmanı da ayrıca kontrol eder,
-- ama son söz veritabanınındır).
-- ---------------------------------------------------------------------------
ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS chk_pp_period_order;
ALTER TABLE progress_payments
    ADD CONSTRAINT chk_pp_period_order
    CHECK (period_start IS NULL OR period_end IS NULL OR period_end >= period_start);

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Aynı taşeron için tarih aralıkları çakışamaz (silinmemiş kayıtlarda).
-- Revizyon kayıtları (revision_of dolu) hariç tutulur: revizyon, düzeltilen
-- dönemle bilinçli olarak aynı aralığı taşır.
ALTER TABLE progress_payments DROP CONSTRAINT IF EXISTS excl_pp_period_overlap;
ALTER TABLE progress_payments
    ADD CONSTRAINT excl_pp_period_overlap
    EXCLUDE USING gist (
        subcontractor_id WITH =,
        daterange(period_start, period_end, '[]') WITH &&
    )
    WHERE (deleted_at IS NULL AND revision_of IS NULL
           AND period_start IS NOT NULL AND period_end IS NOT NULL);

-- ---------------------------------------------------------------------------
-- 4) KESİNTİ KATALOĞU
--
-- Hakediş girişinde kalem atlanmasın diye kesintiler gruplanır ve önceden
-- tanımlı kalem listesinden seçilir. Katalog VERİDİR (kod değil): yeni kesinti
-- türü eklemek için sürüm çıkmaya gerek yoktur — Plan §3 "yetki koda gömülmez,
-- veriye yazılır" ilkesinin kesintilere uygulanmış hâli.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS deduction_catalog (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_code     text NOT NULL
                   CHECK (group_code IN ('Tax','Advance','Retention','Penalty','GoodsService','Adjustment')),
    code           text NOT NULL,              -- teknik kod (ör. WITHHOLDING_INCOME)
    label          text NOT NULL,              -- ekranda görünen ad
    deduction_type text NOT NULL               -- payment_deductions.type eşlemesi
                   CHECK (deduction_type IN ('AdvanceOffset','Retention','Withholding','Tax','OHSPenalty','Other')),
    nature         text NOT NULL
                   CHECK (nature IN ('Offset','Temporary','Permanent')),
    reduces_cost   boolean NOT NULL DEFAULT false,
    default_rate_pct numeric(7,4) NULL,         -- varsa tipik oran (bilgi amaçlı)
    refund_stage   text NULL,                   -- geçici kesintilerde iade aşaması
    note           text NULL,
    sort_order     integer NOT NULL DEFAULT 0,
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_deduction_catalog_code ON deduction_catalog (code);
CREATE INDEX IF NOT EXISTS idx_deduction_catalog_group ON deduction_catalog (group_code) WHERE is_active;

DROP TRIGGER IF EXISTS trg_deduction_catalog_updated_at ON deduction_catalog;
CREATE TRIGGER trg_deduction_catalog_updated_at BEFORE UPDATE ON deduction_catalog
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Kesinti satırı hangi katalog kalemine dayanıyor?
ALTER TABLE payment_deductions
    ADD COLUMN IF NOT EXISTS group_code   text NULL,
    ADD COLUMN IF NOT EXISTS catalog_code text NULL;

CREATE INDEX IF NOT EXISTS idx_pd_group ON payment_deductions (group_code);

-- ---------------------------------------------------------------------------
-- 5) KATALOG BAŞLANGIÇ VERİSİ
-- ---------------------------------------------------------------------------
INSERT INTO deduction_catalog
  (group_code, code, label, deduction_type, nature, reduces_cost, default_rate_pct, refund_stage, note, sort_order)
VALUES
-- ---- Vergi ve yasal kesintiler (kâti, maliyeti azaltmaz) ----
('Tax','WHT_INCOME','Gelir/Kurumlar vergisi stopajı','Withholding','Permanent',false,5.0000,NULL,
 'GVK 42-44 yıllara sari inşaat işleri. Taşeronun vergi mahsubudur; ana yüklenicinin maliyeti brüt kalır.',10),
('Tax','STAMP_DUTY','Damga vergisi','Tax','Permanent',false,0.9480,NULL,
 'Hakediş üzerinden binde 9,48 (yürürlükteki orana göre güncellenir).',20),
('Tax','SGK_DEBT','SGK borcu mahsubu','Tax','Permanent',false,NULL,NULL,
 'İlişiksiz belgesi alınamadığında kurum alacağı için tutulan tutar.',30),
('Tax','EXECUTION_LIEN','İcra / haciz kesintisi','Tax','Permanent',false,NULL,NULL,
 'Mahkeme veya icra dairesi kararıyla taşeron alacağından kesilen tutar.',40),

-- ---- Avans mahsupları (mahsup, maliyeti azaltmaz) ----
('Advance','ADV_CONTRACT','Sözleşme avansı mahsubu','AdvanceOffset','Offset',false,NULL,NULL,
 'Sözleşme kapsamında ödenen avansın dönem brütünden mahsubu (otomatik hesaplanır).',10),
('Advance','ADV_MATERIAL','Malzeme avansı mahsubu','AdvanceOffset','Offset',false,NULL,NULL,NULL,20),
('Advance','ADV_EXTRA','Ek / ara avans mahsubu','AdvanceOffset','Offset',false,NULL,NULL,NULL,30),
('Advance','ADV_EQUIPMENT','Ekipman avansı mahsubu','AdvanceOffset','Offset',false,NULL,NULL,NULL,40),

-- ---- Teminatlar (geçici — iade edilecek, maliyeti azaltmaz) ----
('Retention','RET_PERFORMANCE','Kesin teminat (nakdi)','Retention','Temporary',false,NULL,'Kesin kabul',
 'Kesin kabul ve ilişiksiz belgesi sonrası iade edilir.',10),
('Retention','RET_PROVISIONAL','Geçici kabul teminatı','Retention','Temporary',false,NULL,'Geçici kabul',NULL,20),
('Retention','RET_WHT_GUARANTEE','Stopaj teminatı','Retention','Temporary',false,NULL,'İlişiksiz belgesi',NULL,30),
('Retention','RET_WARRANTY','Bakım / garanti teminatı','Retention','Temporary',false,NULL,'Garanti süresi sonu',NULL,40),

-- ---- Cezalar (kâti, MALİYETİ AZALTIR) ----
('Penalty','PEN_OHS','İSG ceza tutanağı','OHSPenalty','Permanent',true,NULL,NULL,
 'İSG modülünden otomatik önerilir; tutanağa izlenebilir bağ kurulur.',10),
('Penalty','PEN_DELAY','Gecikme cezası (iş programı)','Other','Permanent',true,NULL,NULL,NULL,20),
('Penalty','PEN_DURATION','Süre aşımı cezası','Other','Permanent',true,NULL,NULL,NULL,30),
('Penalty','PEN_QUALITY','Kalite / imalat red cezası','Other','Permanent',true,NULL,NULL,NULL,40),
('Penalty','PEN_ENV','Çevre ve temizlik ihlali cezası','Other','Permanent',true,NULL,NULL,NULL,50),
('Penalty','PEN_ADMIN','İdari para cezası yansıtması','Other','Permanent',true,NULL,NULL,NULL,60),
('Penalty','PEN_DAMAGE','Üçüncü şahıs hasar bedeli','Other','Permanent',true,NULL,NULL,NULL,70),

-- ---- Mal ve hizmet kesintileri (kâti, MALİYETİ AZALTIR) ----
('GoodsService','GS_MEAL','Yemek bedeli','Other','Permanent',true,NULL,NULL,NULL,10),
('GoodsService','GS_LODGING','Konaklama / şantiye barınma','Other','Permanent',true,NULL,NULL,NULL,20),
('GoodsService','GS_TRANSPORT','Personel servisi / ulaşım','Other','Permanent',true,NULL,NULL,NULL,30),
('GoodsService','GS_ELECTRICITY','Elektrik bedeli','Other','Permanent',true,NULL,NULL,NULL,40),
('GoodsService','GS_WATER','Su bedeli','Other','Permanent',true,NULL,NULL,NULL,50),
('GoodsService','GS_UTILITY_OTHER','Doğalgaz / iletişim bedeli','Other','Permanent',true,NULL,NULL,NULL,60),
('GoodsService','GS_MATERIAL','Yüklenici temini malzeme','Other','Permanent',true,NULL,NULL,
 'Demir, çimento, kalıp malzemesi vb. yüklenici tarafından verilen malzeme bedeli.',70),
('GoodsService','GS_CONSUMABLE','Sarf malzeme','Other','Permanent',true,NULL,NULL,NULL,80),
('GoodsService','GS_FUEL','Yakıt bedeli','Other','Permanent',true,NULL,NULL,NULL,90),
('GoodsService','GS_CRANE','Vinç / forklift kullanım bedeli','Other','Permanent',true,NULL,NULL,NULL,100),
('GoodsService','GS_MACHINE','İş makinesi kirası','Other','Permanent',true,NULL,NULL,NULL,110),
('GoodsService','GS_SCAFFOLD','İskele kirası','Other','Permanent',true,NULL,NULL,NULL,120),
('GoodsService','GS_DEBRIS','Moloz / hafriyat nakli','Other','Permanent',true,NULL,NULL,NULL,130),
('GoodsService','GS_CLEANING','Temizlik hizmeti','Other','Permanent',true,NULL,NULL,NULL,140),
('GoodsService','GS_SECURITY','Güvenlik hizmeti','Other','Permanent',true,NULL,NULL,NULL,150),
('GoodsService','GS_LAB','Laboratuvar / deney bedeli','Other','Permanent',true,NULL,NULL,NULL,160),
('GoodsService','GS_PPE','KKD (kişisel koruyucu donanım) bedeli','Other','Permanent',true,NULL,NULL,NULL,170),
('GoodsService','GS_OHS_SHARE','İSG uzmanı / sağlık personeli payı','Other','Permanent',true,NULL,NULL,NULL,180),
('GoodsService','GS_OVERHEAD','Şantiye genel gider katılım payı','Other','Permanent',true,NULL,NULL,NULL,190),
('GoodsService','GS_INSURANCE','Sigorta primi yansıtması','Other','Permanent',true,NULL,NULL,NULL,200),

-- ---- Düzeltme ve mahsuplaşmalar ----
('Adjustment','ADJ_PREV_CORRECTION','Önceki hakediş düzeltmesi','Other','Permanent',false,NULL,NULL,
 'Eksi veya artı yönlü düzeltme; maliyet etkisi kalem bazında belirlenir.',10),
('Adjustment','ADJ_OVERPAYMENT','Fazla ödeme iadesi','Other','Permanent',false,NULL,NULL,NULL,20),
('Adjustment','ADJ_PRICE_DIFF','Fiyat farkı mahsubu','Other','Permanent',false,NULL,NULL,NULL,30),
('Adjustment','ADJ_OUT_OF_SCOPE','Sözleşme dışı iş mahsubu','Other','Permanent',false,NULL,NULL,NULL,40)
ON CONFLICT (code) DO NOTHING;

-- Mevcut kesinti satırlarına grup ata (geriye dönük).
-- Kilit trigger'ı şema geçişi boyunca devre dışı (bkz. 000012 açıklaması).
ALTER TABLE payment_deductions DISABLE TRIGGER trg_pd_lock;

UPDATE payment_deductions SET group_code = 'Advance'      WHERE type = 'AdvanceOffset' AND group_code IS NULL;
UPDATE payment_deductions SET group_code = 'Retention'    WHERE type = 'Retention'     AND group_code IS NULL;
UPDATE payment_deductions SET group_code = 'Tax'          WHERE type IN ('Withholding','Tax') AND group_code IS NULL;
UPDATE payment_deductions SET group_code = 'Penalty'      WHERE type = 'OHSPenalty'    AND group_code IS NULL;
UPDATE payment_deductions SET group_code = 'GoodsService' WHERE type = 'Other'         AND group_code IS NULL;

ALTER TABLE payment_deductions ENABLE TRIGGER trg_pd_lock;
