-- Faz 23 — Demo seed verisi (DEMO-04 Nilüfer Şehir Hastanesi Kompleksi)
-- DEMO-04 projesi yoksa sabit UUID ile oluşturulur; varsa mevcut ID kullanılır.
-- ON CONFLICT DO NOTHING ile idempotent; tekrar çalıştırılabilir.

DO $$
DECLARE
  v_proj  uuid;
  v_admin uuid;
  v_role  uuid;
BEGIN
  -- Admin kullanıcı
  SELECT id INTO v_admin FROM users WHERE deleted_at IS NULL LIMIT 1;

  -- PM rolü
  SELECT id INTO v_role FROM roles WHERE code = 'ProjectManager' LIMIT 1;
  IF v_role IS NULL THEN
    SELECT id INTO v_role FROM roles LIMIT 1;
  END IF;

  -- DEMO-04 projesini bul (code ile)
  SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-04' AND deleted_at IS NULL;

  IF v_proj IS NULL THEN
    -- Sabit UUID ile oluştur; UUID çakışırsa code ile tekrar al
    BEGIN
      INSERT INTO projects (id, code, name, location, client_name, budget_total,
                            currency, start_date, end_date, status)
      VALUES ('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f',
              'DEMO-04', 'Nilüfer Sehir Hastanesi Kompleksi',
              'Bursa / Nilufer', 'Turkiye Kamu Hastaneleri Kurumu (TKHK)',
              320000000, 'TRY', '2025-01-01', '2027-12-31', 'Active')
      RETURNING id INTO v_proj;
    EXCEPTION WHEN unique_violation THEN
      SELECT id INTO v_proj FROM projects WHERE code = 'DEMO-04' AND deleted_at IS NULL;
    END;

    -- Proje üyeliği
    IF v_admin IS NOT NULL AND v_role IS NOT NULL THEN
      INSERT INTO project_members (project_id, user_id, role_id)
      VALUES (v_proj, v_admin, v_role)
      ON CONFLICT DO NOTHING;
    END IF;
  END IF;

  -- ── Tasarım ve Projeler ──────────────────────────────────────────────────
  INSERT INTO project_design_docs
      (project_id, disiplin, poz_no, baslik, rev_no, tarih, durum, aciklama, sira)
  VALUES
  (v_proj,'Mimari','A.000','Vaziyet ve Yerlesim Plani','C','2024-03-15','onaylı','Onaylandi, insaata esas',10),
  (v_proj,'Mimari','A.101','Bodrum Kat Mimari Plani','D','2024-04-20','onaylı','Teknik Detaylar Tamamlandi',20),
  (v_proj,'Mimari','A.102','Zemin Kat Mimari Plani','E','2024-05-10','onaylı','Son Revizyon Onaylandi',30),
  (v_proj,'Mimari','A.103','1-4 Normal Kat Mimari Plani','B','2024-06-05','incelemede','Islah notlari bekleniyor',40),
  (v_proj,'Mimari','A.104','5-8 Normal Kat Mimari Plani','A','2024-07-01','revizyon_gerekli','Koridor genisligi revize edilecek',50),
  (v_proj,'Mimari','A.201','Cephe Detaylari','B','2024-05-20','onaylı','Cephe sistemi onaylandi',60),
  (v_proj,'Mimari','A.301','Merdiven ve Asansor Detaylari','A','2024-06-15','incelemede','Bakanlik Engelli Erisilebilirligi incelemesinde',70),
  (v_proj,'Statik','S.001','Zemin Etud ve Sondaj Raporu','1','2024-01-10','onaylı','TMMOB onaylı zemin raporu',10),
  (v_proj,'Statik','S.101','Temel Sistemi Plani','C','2024-03-20','onaylı','Kazik hesaplari revize edildi',20),
  (v_proj,'Statik','S.201','Bodrum Perde ve Kolon Yerlesimi','B','2024-04-15','onaylı','Betonarme hesaplariyla uyumlu',30),
  (v_proj,'Statik','S.202','Zemin-8 Kat Kiriş ve Kolon Aplikasyonu','B','2024-05-25','incelemede','Hesap raporu teslim edildi, onay bekleniyor',40),
  (v_proj,'Statik','S.301','Celik Cati Makaslari ve Baglanti Detaylari','A','2024-07-10','taslak','Tasarim devam ediyor',50),
  (v_proj,'Mekanik','M.001','Sihhi Tesisat Sematik Plani','B','2024-04-05','onaylı','Tesisat ve boru guzergahlari onaylandi',10),
  (v_proj,'Mekanik','M.101','Yangin Sondurme Sistemi Plani','B','2024-04-25','onaylı','Itfaiye onayi alindi',20),
  (v_proj,'Mekanik','M.201','HVAC Hava Kanallari Plani','A','2024-06-01','incelemede','Enerji kimlik belgesi hazirlaniyor',30),
  (v_proj,'Mekanik','M.301','Tibbi Gaz Sistemi (O2, N2O, Vakum)','A','2024-06-20','taslak','Uzman firma tasarimi bekleniyor',40),
  (v_proj,'Elektrik','E.001','AG-YG Guc Dagitim Sematigi','C','2024-03-10','onaylı','Trafo ve jenerator gucu netlesti',10),
  (v_proj,'Elektrik','E.101','Aydinlatma ve Acil Aydinlatma Plani','B','2024-05-05','onaylı','EN 12464 normuna uygun',20),
  (v_proj,'Elektrik','E.201','Yapisal Kablo Altyapisi (SCS)','A','2024-06-10','incelemede','Cat6A tercih onayi bekleniyor',30),
  (v_proj,'Elektrik','E.301','CCTV ve Erisim Kontrol Sistemi','0','2024-07-15','taslak','Guvenlik firmasi ile koordinasyon suruyor',40),
  (v_proj,'İç Mimari','IC.001','Zemin Kat Resepsiyon ve Bekleme Alani','B','2024-05-15','onaylı','Malzeme listesi kesinlestirildi',10),
  (v_proj,'İç Mimari','IC.101','Cerrahi Blok Ic Mekan Tasarimi','A','2024-06-25','incelemede','Hijyen sartnamesi uyum kontrolu yapiliyor',20),
  (v_proj,'İç Mimari','IC.201','Hasta Odalari Detay Plani','0',NULL,'taslak','Konsept asamasinda',30),
  (v_proj,'Peyzaj','P.001','Dis Mekan Duzenleme Plani','A','2024-05-30','incelemede','Belediye onayi bekleniyor',10),
  (v_proj,'Peyzaj','P.101','Otopark ve Yaya Akslari','A','2024-06-08','taslak','Trafik yogunluk raporu alindi',20)
  ON CONFLICT DO NOTHING;

  -- ── Personel ─────────────────────────────────────────────────────────────
  INSERT INTO project_personnel (project_id, ad_soyad, gorev, firma, is_aktif, sira)
  VALUES
  (v_proj,'Mehmet Yilmaz','Formen','Ana Yuklenici',true,10),
  (v_proj,'Ali Kaya','Isci','Ana Yuklenici',true,20),
  (v_proj,'Hasan Demir','Isci','Ana Yuklenici',true,30),
  (v_proj,'Ibrahim Celik','Isci','Ana Yuklenici',true,40),
  (v_proj,'Mustafa Sahin','Teknisyen','Ana Yuklenici',true,50),
  (v_proj,'Ahmet Kurt','Isci','Alt Yuklenici A',true,60),
  (v_proj,'Omer Aksoy','Isci','Alt Yuklenici A',true,70),
  (v_proj,'Ramazan Yildirim','Isci','Alt Yuklenici A',true,80),
  (v_proj,'Serkan Ozturk','Formen','Alt Yuklenici B',true,90),
  (v_proj,'Burak Arslan','Isci','Alt Yuklenici B',true,100),
  (v_proj,'Emre Polat','Isci','Alt Yuklenici B',true,110),
  (v_proj,'Kemal Erdogan','Muhendis','Ana Yuklenici',true,120),
  (v_proj,'Fatih Gul','Teknisyen','Alt Yuklenici C',true,130),
  (v_proj,'Okan Tekin','Isci','Alt Yuklenici C',true,140),
  (v_proj,'Baris Cinar','Isci','Ana Yuklenici',false,150)
  ON CONFLICT DO NOTHING;

  -- ── Tedarikçi Ekstreler ──────────────────────────────────────────────────
  INSERT INTO supplier_statements
      (project_id, tedarikci_adi, ekstre_no, ekstre_tarihi, vade_tarihi,
       toplam_tutar, odenen_tutar, para_birimi, odeme_durumu, aciklama)
  VALUES
  (v_proj,'Turk Demir Celik A.S.','TDC-2024-0042','2024-03-01','2024-04-01',4250000,4250000,'TRY','odendi','Mart insaat demiri'),
  (v_proj,'Turk Demir Celik A.S.','TDC-2024-0071','2024-04-01','2024-05-01',5180000,5180000,'TRY','odendi','Nisan insaat demiri'),
  (v_proj,'Turk Demir Celik A.S.','TDC-2024-0098','2024-05-01','2024-06-01',4920000,2500000,'TRY','kismi_odendi','Mayis teslivati'),
  (v_proj,'Anadolu Beton Sanayi','ABS-2024-0115','2024-03-15','2024-04-15',8600000,8600000,'TRY','odendi','Mart hazir beton'),
  (v_proj,'Anadolu Beton Sanayi','ABS-2024-0143','2024-04-15','2024-05-15',9250000,9250000,'TRY','odendi','Nisan hazir beton'),
  (v_proj,'Anadolu Beton Sanayi','ABS-2024-0178','2024-05-15','2024-06-15',7800000,0,'TRY','bekliyor','Mayis hazir beton'),
  (v_proj,'Delta Mekanik Tesisat','DMT-2024-0023','2024-04-20','2024-05-20',3150000,3150000,'TRY','odendi','HVAC ekipman'),
  (v_proj,'Delta Mekanik Tesisat','DMT-2024-0041','2024-05-20','2024-06-20',2800000,0,'TRY','bekliyor','Yangin sondurme borulama'),
  (v_proj,'Ege Elektrik Malzemeleri','EEM-2024-0067','2024-04-10','2024-05-10',1920000,1920000,'TRY','odendi','Guc kabloları ve panolar'),
  (v_proj,'Ege Elektrik Malzemeleri','EEM-2024-0089','2024-05-10','2024-06-10',2340000,1170000,'TRY','kismi_odendi','Aydinlatma armaturleri'),
  (v_proj,'Yapi Kimyasallari Ltd.','YKL-2024-0034','2024-04-05','2024-05-05',680000,680000,'TRY','odendi','Beton katki maddeleri'),
  (v_proj,'Yapi Kimyasallari Ltd.','YKL-2024-0052','2024-05-05','2024-06-05',720000,0,'TRY','bekliyor','Su yalitim malzemeleri')
  ON CONFLICT DO NOTHING;

END;
$$;
