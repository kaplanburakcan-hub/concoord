-- Migration 000030 — Şantiye Kasa Harcaması tablosu + DEMO-04 depo/kasa örnek verisi
--
-- Kullanıcı geri bildirimi: haftalık rapor görünümünde Aktif Taşeron Listesi,
-- Satın Alma ve Şantiye Kasa Harcaması bölümleri eksikti; Depo-Stok bölümü
-- kodda vardı ama DEMO-04 projesinde hiç depo kalemi tanımlı olmadığından boş
-- görünüyordu. Bu migration:
--   1) daily_cash_expenses tablosunu açar (manpower/equipment/work_entries ile
--      aynı desen: daily_report_id'ye bağlı, Submitted kilidine tabi satır).
--   2) DEMO-04 için örnek depo kalemleri + hareketleri ekler.
--   3) DEMO-04'ün Hafta 31-32 taslak günlük raporlarına örnek kasa harcaması
--      satırları ekler (Submitted rapora dokunulmaz).

-- ---------------------------------------------------------------------------
-- 1. daily_cash_expenses
-- ---------------------------------------------------------------------------
CREATE TABLE daily_cash_expenses (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_report_id uuid NOT NULL REFERENCES daily_reports (id) ON DELETE RESTRICT,
    description     text NOT NULL,
    category        text NOT NULL DEFAULT 'Diğer',
    amount          numeric(12,2) NOT NULL CHECK (amount >= 0),
    receipt_no      text NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz NULL,
    row_version     integer NOT NULL DEFAULT 1
);
CREATE INDEX idx_daily_cash_expenses_report ON daily_cash_expenses (daily_report_id);

CREATE TRIGGER trg_daily_cash_expenses_updated_at BEFORE UPDATE ON daily_cash_expenses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_dce_lock BEFORE INSERT OR UPDATE OR DELETE ON daily_cash_expenses
    FOR EACH ROW EXECUTE FUNCTION lock_submitted_daily_children();

-- ---------------------------------------------------------------------------
-- 2 + 3. DEMO-04 örnek verisi
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  v_proj uuid;
  v_cimento uuid;
  v_demir uuid;
  v_kalip uuid;
  v_izolasyon uuid;
  v_boru uuid;
  dr RECORD;
  i INTEGER;
BEGIN
  SELECT id INTO v_proj FROM projects WHERE code='DEMO-04' AND deleted_at IS NULL;
  IF v_proj IS NULL THEN RETURN; END IF;

  -- Depo kalemleri (mevcut_miktar = hafta 32 sonu itibarıyla; aşağıdaki
  -- hareketlerle tutarlı olacak şekilde elle hesaplanmıştır).
  INSERT INTO site_warehouse_items (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
  VALUES (v_proj, 'Çimento (CEM I 42.5R)', 'İnşaat Malzemesi', 'ton', 38.0, 20.0, 1)
  RETURNING id INTO v_cimento;

  INSERT INTO site_warehouse_items (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
  VALUES (v_proj, 'İnşaat Demiri (B420C, Ø12-16)', 'İnşaat Malzemesi', 'ton', 12.5, 15.0, 2)
  RETURNING id INTO v_demir;

  INSERT INTO site_warehouse_items (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
  VALUES (v_proj, 'Kalıp Kontrplak (18mm)', 'Kalıp Malzemesi', 'adet', 210, 100, 3)
  RETURNING id INTO v_kalip;

  INSERT INTO site_warehouse_items (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
  VALUES (v_proj, 'Su Yalıtım Membranı', 'İzolasyon', 'rulo', 24, 30.0, 4)
  RETURNING id INTO v_izolasyon;

  INSERT INTO site_warehouse_items (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, sira)
  VALUES (v_proj, 'Yangın Tesisatı Boru (DN50)', 'Mekanik Malzeme', 'm', 84, 50.0, 5)
  RETURNING id INTO v_boru;

  -- Hareketler — Hafta 31 (27.07-02.08) ve Hafta 32 (03.08-09.08).
  INSERT INTO site_warehouse_movements (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama) VALUES
    (v_proj, v_cimento,    'giris', 25.0, '2026-07-28', 'İRS-3301', 'Bursa Çimento sevkiyatı'),
    (v_proj, v_cimento,    'cikis', 18.0, '2026-07-29', NULL,       'Kolon beton dökümü sarfiyatı'),
    (v_proj, v_cimento,    'cikis', 14.0, '2026-08-05', NULL,       'Döşeme betonu sarfiyatı'),
    (v_proj, v_demir,      'giris', 10.0, '2026-07-27', 'İRS-3298', 'Demir çelik sevkiyatı'),
    (v_proj, v_demir,      'cikis',  6.5, '2026-07-30', NULL,       'Demir imalatı sarfiyatı'),
    (v_proj, v_kalip,      'giris', 120,  '2026-07-27', 'İRS-3299', 'Kalıp kontrplak sevkiyatı'),
    (v_proj, v_kalip,      'cikis',  60,  '2026-08-04', NULL,       'Döşeme kalıbı kurulumu'),
    (v_proj, v_izolasyon,  'giris', 40,   '2026-07-31', 'İRS-3305', 'İzolasyon malzemesi sevkiyatı'),
    (v_proj, v_izolasyon,  'cikis', 16,   '2026-08-06', NULL,       'Bodrum perde izolasyonu'),
    (v_proj, v_boru,       'giris', 100,  '2026-07-29', 'İRS-3302', 'Yangın tesisatı boru sevkiyatı'),
    (v_proj, v_boru,       'cikis', 16,   '2026-08-07', NULL,       'B Blok bodrum tesisatı');

  -- Şantiye Kasa Harcaması — taslak günlük raporlara günde 1-2 küçük kalem.
  FOR dr IN
    SELECT id, report_date FROM daily_reports
    WHERE project_id = v_proj AND deleted_at IS NULL AND status != 'Submitted'
      AND report_date BETWEEN '2026-07-27' AND '2026-08-09'
    ORDER BY report_date
  LOOP
    INSERT INTO daily_cash_expenses (daily_report_id, description, category, amount, receipt_no) VALUES
      (dr.id, 'Şantiye personeli yemek bedeli', 'Yemek', 850 + (random()*150)::int, 'MKB-' || to_char(dr.report_date,'MMDD') || '-1');

    -- Haftada birkaç gün ek kalem (yakıt/nakliye/küçük malzeme) ekle.
    IF extract(dow from dr.report_date) IN (1,3,5) THEN
      INSERT INTO daily_cash_expenses (daily_report_id, description, category, amount, receipt_no) VALUES
        (dr.id, 'Jeneratör yakıt ikmali', 'Yakıt', 1200 + (random()*400)::int, 'MKB-' || to_char(dr.report_date,'MMDD') || '-2');
    END IF;
    IF extract(dow from dr.report_date) IN (2,4) THEN
      INSERT INTO daily_cash_expenses (daily_report_id, description, category, amount, receipt_no) VALUES
        (dr.id, 'Küçük el aleti / sarf malzeme alımı', 'Küçük Malzeme', 320 + (random()*180)::int, 'MKB-' || to_char(dr.report_date,'MMDD') || '-2');
    END IF;
  END LOOP;

END $$;
