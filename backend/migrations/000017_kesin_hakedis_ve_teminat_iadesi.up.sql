-- ============================================================================
-- Faz 11 / 000017 — Kesin hakediş ve geçici kesinti (teminat) iade akışı
-- ============================================================================
-- Üç eksik giderilir:
--
-- 1) KESİN HAKEDİŞ. Bir hakediş "kesin hakediş" olarak işaretlenebilir. Bu, işin
--    tamamlandığı ve hesabın kapatıldığı anlamına gelir; teminat iadeleri ve
--    kalan mahsuplar bu hakedişte sonuçlandırılır. Raporlama (aylık rapor, EVM,
--    taşeron hesap özeti) kesin hakedişi ayrı ele alır.
--
-- 2) GEÇİCİ KABUL BELGESİ. Kesin hakediş, geçici kabulün yapılmış olmasını
--    gerektirir. Belge olmadan kesin hakediş düzenlenemez: bu, hem sözleşme
--    gereği hem de teminat iadesinin dayanağıdır. (Zorunluluk uygulama
--    katmanında denetlenir; mevcut kayıtlar geriye dönük bozulmaz.)
--
-- 3) TEMİNAT İADESİ. Bugüne dek geçici nitelikli kesintiler (teminat) birikiyor
--    ama iade edilemiyordu — sözleşmeden doğan bir yükümlülük sistemde takipsiz
--    kalıyordu. İadeler artık kayıt altına alınır ve hakedişte İLAVE olarak
--    ödenecek tutarı ARTIRIR (kesinti gibi azaltmaz).
--
--    Teminat bakiyesi türetilir, ayrı bir alanda tutulmaz:
--        bakiye = Σ(geçici kesintiler) − Σ(iadeler)
--    Böylece iki kaynak arasında tutarsızlık oluşamaz.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1) Kesin hakediş ve geçici kabul belgesi
-- ---------------------------------------------------------------------------
ALTER TABLE progress_payments
    ADD COLUMN IF NOT EXISTS is_final boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS provisional_acceptance_document_id uuid NULL
        REFERENCES documents (id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS provisional_acceptance_date date NULL,
    ADD COLUMN IF NOT EXISTS total_additions numeric(18,2) NOT NULL DEFAULT 0;

COMMENT ON COLUMN progress_payments.is_final IS
    'Kesin hakediş: hesap kapanır, teminat iadeleri bu hakedişte sonuçlandırılır';
COMMENT ON COLUMN progress_payments.provisional_acceptance_document_id IS
    'Geçici kabul belgesi — kesin hakedişte zorunlu (uygulama katmanı denetler)';
COMMENT ON COLUMN progress_payments.total_additions IS
    'İlaveler toplamı (teminat iadeleri vb.) — ödenecek tutarı artırır';

-- Bir taşeronun aynı sözleşmesinde birden fazla kesin hakediş olamaz.
CREATE UNIQUE INDEX IF NOT EXISTS uq_pp_final_per_sub
    ON progress_payments (subcontractor_id)
    WHERE is_final AND deleted_at IS NULL AND status <> 'Rejected';

-- ---------------------------------------------------------------------------
-- 2) Geçici kesinti iadeleri
--
-- İade, hangi hakedişte ödendiğine bağlanır (progress_payment_id). Hakedişten
-- bağımsız iade (ör. banka teminat mektubu iadesi) da kaydedilebilir; bu durumda
-- alan boş kalır ve yalnızca bakiyeyi düşürür.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS deduction_refunds (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          uuid NOT NULL REFERENCES projects (id)         ON DELETE RESTRICT,
    subcontractor_id    uuid NOT NULL REFERENCES subcontractors (id)   ON DELETE RESTRICT,
    progress_payment_id uuid NULL     REFERENCES progress_payments (id) ON DELETE RESTRICT,
    catalog_code        text NULL,                 -- hangi kesinti kalemi iade ediliyor
    description         text NOT NULL,
    amount              numeric(18,2) NOT NULL CHECK (amount > 0),
    stage               text NOT NULL
                        CHECK (stage IN ('ProvisionalAcceptance','FinalAcceptance',
                                         'ClearanceCertificate','WarrantyEnd','Other')),
    document_id         uuid NULL REFERENCES documents (id) ON DELETE RESTRICT, -- dayanak belge
    note                text NULL,
    created_by          uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz NULL,
    row_version         integer NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_refunds_sub
    ON deduction_refunds (subcontractor_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_refunds_payment
    ON deduction_refunds (progress_payment_id) WHERE progress_payment_id IS NOT NULL;

DROP TRIGGER IF EXISTS trg_refunds_updated_at ON deduction_refunds;
CREATE TRIGGER trg_refunds_updated_at BEFORE UPDATE ON deduction_refunds
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE deduction_refunds IS
    'Geçici kesinti (teminat) iadeleri. Hakedişe bağlıysa ödenecek tutarı artırır.';
COMMENT ON COLUMN deduction_refunds.stage IS
    'İade aşaması: geçici kabul, kesin kabul, ilişiksiz belgesi, garanti sonu';

-- ---------------------------------------------------------------------------
-- 3) Teminat bakiyesi görünümü
--
-- Bakiye türetilir (kesintiler − iadeler); ayrı alanda tutulmaz ki iki kaynak
-- arasında tutarsızlık oluşamasın. Yalnızca KESİNLEŞMİŞ hakedişlerin kesintileri
-- sayılır: taslak hakediş henüz bir yükümlülük doğurmaz.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW v_retention_balance AS
SELECT
    s.id                                   AS subcontractor_id,
    s.project_id,
    s.company_name,
    COALESCE(d.withheld, 0)                AS total_withheld,
    COALESCE(r.refunded, 0)                AS total_refunded,
    COALESCE(d.withheld, 0) - COALESCE(r.refunded, 0) AS balance
FROM subcontractors s
LEFT JOIN (
    SELECT pp.subcontractor_id, sum(pd.amount) AS withheld
    FROM payment_deductions pd
    JOIN progress_payments pp ON pp.id = pd.progress_payment_id
    WHERE pd.nature = 'Temporary'
      AND pp.status = 'Finalized'
      AND pp.deleted_at IS NULL
    GROUP BY pp.subcontractor_id
) d ON d.subcontractor_id = s.id
LEFT JOIN (
    SELECT subcontractor_id, sum(amount) AS refunded
    FROM deduction_refunds
    WHERE deleted_at IS NULL
    GROUP BY subcontractor_id
) r ON r.subcontractor_id = s.id
WHERE s.deleted_at IS NULL;

COMMENT ON VIEW v_retention_balance IS
    'Taşeron bazında teminat bakiyesi: kesinleşmiş hakedişlerdeki geçici kesintiler eksi iadeler';
