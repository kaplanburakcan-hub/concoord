-- ============================================================================
-- Faz 11 / 000012 — Kesinti sınıflandırması ve stopaj (yıllara sari işler)
-- ============================================================================
-- İki eksik giderilir:
--
-- 1) KESİNTİ NİTELİĞİ. Kesintiler tip olarak tutuluyordu ama "iade edilecek mi"
--    ve "işin maliyetini azaltır mı" bilgisi kodlanmamıştı. Üç nitelik:
--      · Offset    (mahsup)  — avans: daha önce ÖDENMİŞ tutarın geri alınması.
--                              İade yükümlülüğü yok, maliyeti azaltmaz.
--      · Temporary (geçici)  — teminat: taşerondan tutulur, kabulde İADE EDİLİR.
--                              Maliyeti azaltmaz (borç niteliğinde).
--      · Permanent (kâti)    — iade edilmez: yemek, konaklama, elektrik, malzeme,
--                              İSG cezası, gecikme cezası, stopaj.
--    Ayrıca `reduces_cost`: kesinti gerçekten ana yüklenicinin maliyetini
--    azaltıyor mu? Yemek/malzeme/İSG cezası → evet (taşerona verilen mal/hizmet
--    bedeli). Avans/teminat → hayır (finansman). Stopaj → HAYIR: taşeronun gelir
--    vergisi mahsubudur, kaynakta kesilip onun adına yatırılır; ana yüklenicinin
--    işe maliyeti brüt tutar olarak kalır.
--
-- 2) STOPAJ (GVK Md. 42-44 — yıllara sari inşaat işleri). Aynı takvim yılında
--    başlayıp biten işlerde stopaj kesilmez; sonraki yıla sarkan ve o yılda en az
--    bir hakediş düzenlenen işlerde kesilir. Bu yüzden stopaj sözleşme düzeyinde
--    varsayılanı olan, HAKEDİŞ DÜZEYİNDE elle açılıp kapatılabilen bir kalemdir.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1) Kesinti niteliği
-- ---------------------------------------------------------------------------
ALTER TABLE payment_deductions
    ADD COLUMN IF NOT EXISTS nature       text    NOT NULL DEFAULT 'Permanent',
    ADD COLUMN IF NOT EXISTS reduces_cost boolean NOT NULL DEFAULT true;

-- Stopaj ayrı bir tip: 'Tax' genel vergi/damga vb. için kalır.
ALTER TABLE payment_deductions DROP CONSTRAINT IF EXISTS payment_deductions_type_check;
ALTER TABLE payment_deductions
    ADD CONSTRAINT payment_deductions_type_check
    CHECK (type IN ('AdvanceOffset','Retention','Withholding','Tax','OHSPenalty','Other'));

ALTER TABLE payment_deductions DROP CONSTRAINT IF EXISTS chk_pd_nature;
ALTER TABLE payment_deductions
    ADD CONSTRAINT chk_pd_nature CHECK (nature IN ('Offset','Temporary','Permanent'));

-- Mevcut kayıtların geriye dönük sınıflandırması.
--
-- NOT: trg_pd_lock, kesinleşmiş hakedişlerin kesinti satırlarının değişmesini
-- engeller. Bu doğru bir korumadır ancak ŞEMA GEÇİŞİ o kuralın istisnasıdır:
-- tutarlar değişmiyor, yalnızca yeni sınıflandırma kolonları dolduruluyor.
-- Trigger geçiş süresince devre dışı bırakılır ve hemen geri açılır.
ALTER TABLE payment_deductions DISABLE TRIGGER trg_pd_lock;

UPDATE payment_deductions SET nature = 'Offset',    reduces_cost = false WHERE type = 'AdvanceOffset';
UPDATE payment_deductions SET nature = 'Temporary', reduces_cost = false WHERE type = 'Retention';
UPDATE payment_deductions SET nature = 'Permanent', reduces_cost = false WHERE type IN ('Withholding','Tax');
UPDATE payment_deductions SET nature = 'Permanent', reduces_cost = true  WHERE type IN ('OHSPenalty','Other');

ALTER TABLE payment_deductions ENABLE TRIGGER trg_pd_lock;

COMMENT ON COLUMN payment_deductions.nature IS
    'Offset=avans mahsubu, Temporary=iade edilecek (teminat), Permanent=kâti';
COMMENT ON COLUMN payment_deductions.reduces_cost IS
    'true ise EVM Gerçekleşen Maliyet (AC) hesabından düşülür (mal/hizmet bedeli)';

-- AC hesabı bu kolonu tarar (Finalized hakedişler için).
CREATE INDEX IF NOT EXISTS idx_pd_reduces_cost
    ON payment_deductions (progress_payment_id) WHERE reduces_cost;

-- ---------------------------------------------------------------------------
-- 2) Sözleşme: yıllara sari mi + stopaj oranı
-- ---------------------------------------------------------------------------
ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS is_multi_year   boolean      NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS withholding_pct numeric(5,2) NOT NULL DEFAULT 5.00;

ALTER TABLE contracts DROP CONSTRAINT IF EXISTS chk_contracts_withholding;
ALTER TABLE contracts
    ADD CONSTRAINT chk_contracts_withholding
    CHECK (withholding_pct >= 0 AND withholding_pct <= 100);

COMMENT ON COLUMN contracts.is_multi_year IS
    'Yıllara sari inşaat işi (GVK 42-44): true ise hakedişlerde stopaj varsayılan olarak uygulanır';

-- Mevcut sözleşmeler için makul varsayılan: projenin başlangıç ve bitiş yılı
-- farklıysa iş yıllara saridir. Kullanıcı sonradan düzeltebilir.
UPDATE contracts c
   SET is_multi_year = true
  FROM projects p
 WHERE p.id = c.project_id
   AND p.start_date IS NOT NULL AND p.end_date IS NOT NULL
   AND date_part('year', p.end_date) > date_part('year', p.start_date);

-- ---------------------------------------------------------------------------
-- 3) Hakediş: stopaj uygulansın mı (NULL = sözleşme varsayılanını izle)
-- ---------------------------------------------------------------------------
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS withholding_applied boolean NULL;

COMMENT ON COLUMN progress_payments.withholding_applied IS
    'NULL=sözleşme varsayılanı (is_multi_year), true/false=bu hakediş için elle geçersiz kılma';
