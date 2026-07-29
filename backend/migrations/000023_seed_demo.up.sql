-- Faz 23 — Demo seed verisi (DEMO-04 Nilüfer Şehir Hastanesi Kompleksi)
-- project_id: 06802be9-d6e4-4bc8-abb4-6aaa6bfc840f
-- INSERT ... ON CONFLICT DO NOTHING ile idempotent; tekrar çalıştırılabilir.

-- ── Tasarım ve Projeler ──────────────────────────────────────────────────────
INSERT INTO project_design_docs
    (project_id, disiplin, poz_no, baslik, rev_no, tarih, durum, aciklama, sira)
VALUES
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.000','Vaziyet ve Yerlesim Plani','C','2024-03-15','onaylı','Onaylandı, inşaata esas',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.101','Bodrum Kat Mimari Plani','D','2024-04-20','onaylı','Teknik Detaylar Tamamlandi',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.102','Zemin Kat Mimari Plani','E','2024-05-10','onaylı','Son Revizyon Onaylandi',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.103','1-4 Normal Kat Mimari Plani','B','2024-06-05','incelemede','Islah notlari bekleniyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.104','5-8 Normal Kat Mimari Plani','A','2024-07-01','revizyon_gerekli','Koridor genisligi revize edilecek',50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.201','Cephe Detaylari','B','2024-05-20','onaylı','Cephe sistemi onaylandi',60),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.301','Merdiven ve Asansor Detaylari','A','2024-06-15','incelemede','Bakanlik Engelli Erisilebilirligi incelemesinde',70),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.001','Zemin Etud ve Sondaj Raporu','1','2024-01-10','onaylı','TMMOB onaylı zemin raporu',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.101','Temel Sistemi Plani (Kaz. Burgu+Raft)','C','2024-03-20','onaylı','Kazik hesaplari revize edildi',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.201','Bodrum Perde ve Kolon Yerlesimi','B','2024-04-15','onaylı','Betonarme hesaplariyla uyumlu',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.202','Zemin-8 Kat Kiriş ve Kolon Aplikasyonu','B','2024-05-25','incelemede','Hesap raporu teslim edildi, onay bekleniyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.301','Çelik Çatı Makasları ve Bağlantı Detaylari','A','2024-07-10','taslak','Tasarım devam ediyor',50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.001','Sıhhi Tesisat Şematik Planı','B','2024-04-05','onaylı','Tesisat ve boru güzergahlari onaylandi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.101','Yangın Söndürme Sistemi Planı','B','2024-04-25','onaylı','Itfaiye onayı alındı',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.201','HVAC Hava Kanallari Planı','A','2024-06-01','incelemede','Enerji kimlik belgesi hazirlanıyor',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.301','Tıbbi Gaz Sistemi (O2, N2O, Vakum)','A','2024-06-20','taslak','Uzman firma tasarımı bekleniyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.001','AG-YG Güc Dagitim Sematigi','C','2024-03-10','onaylı','Trafo ve jeneratör gücü netlestirildi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.101','Aydinlatma ve Acil Aydinlatma Plani','B','2024-05-05','onaylı','EN 12464 normuna uygun',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.201','Yapisal Kablo Altyapisi (SCS)','A','2024-06-10','incelemede','Cat6A tercih onayı bekleniyor',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.301','CCTV ve Erişim Kontrol Sistemi','0','2024-07-15','taslak','Güvenlik firması ile koordinasyon surüyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.001','Zemin Kat Resepsiyon ve Bekleme Alanı','B','2024-05-15','onaylı','Malzeme listesi kesinlestirildi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.101','Cerrahi Blok İç Mekan Tasarimi','A','2024-06-25','incelemede','Hijyen sartnamesi uyum kontrolü yapılıyor',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.201','Hasta Odaları Detay Planı','0',NULL,'taslak','Konsept aşamasında',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','P.001','Dış Mekan Düzenleme Planı','A','2024-05-30','incelemede','Belediye onayı bekleniyor',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','P.101','Otopark ve Yaya Aksları','A','2024-06-08','taslak','Trafik yoğunluk raporu alındı',20)
ON CONFLICT DO NOTHING;

-- ── Personel ─────────────────────────────────────────────────────────────────
INSERT INTO project_personnel (project_id, ad_soyad, gorev, firma, is_aktif, sira)
VALUES
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mehmet Yilmaz','Formen','Ana Yuklenici',true,10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ali Kaya','Isci','Ana Yuklenici',true,20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Hasan Demir','Isci','Ana Yuklenici',true,30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ibrahim Celik','Isci','Ana Yuklenici',true,40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mustafa Sahin','Teknisyen','Ana Yuklenici',true,50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ahmet Kurt','Isci','Alt Yuklenici A',true,60),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Omer Aksoy','Isci','Alt Yuklenici A',true,70),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ramazan Yildirim','Isci','Alt Yuklenici A',true,80),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Serkan Ozturk','Formen','Alt Yuklenici B',true,90),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Burak Arslan','Isci','Alt Yuklenici B',true,100),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Emre Polat','Isci','Alt Yuklenici B',true,110),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Kemal Erdogan','Muhendis','Ana Yuklenici',true,120),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Fatih Gul','Teknisyen','Alt Yuklenici C',true,130),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Okan Tekin','Isci','Alt Yuklenici C',true,140),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Baris Cinar','Isci','Ana Yuklenici',false,150)
ON CONFLICT DO NOTHING;

-- ── Tedarikçi Ekstreler ──────────────────────────────────────────────────────
INSERT INTO supplier_statements
    (project_id, tedarikci_adi, ekstre_no, ekstre_tarihi, vade_tarihi,
     toplam_tutar, odenen_tutar, para_birimi, odeme_durumu, aciklama)
VALUES
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0042','2024-03-01','2024-04-01',4250000,4250000,'TRY','odendi','Mart insaat demiri teslivati'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0071','2024-04-01','2024-05-01',5180000,5180000,'TRY','odendi','Nisan insaat demiri teslivati'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0098','2024-05-01','2024-06-01',4920000,2500000,'TRY','kismi_odendi','Mayis teslivati - kismi odeme'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0115','2024-03-15','2024-04-15',8600000,8600000,'TRY','odendi','Mart hazir beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0143','2024-04-15','2024-05-15',9250000,9250000,'TRY','odendi','Nisan hazir beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0178','2024-05-15','2024-06-15',7800000,0,'TRY','bekliyor','Mayis hazir beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Delta Mekanik Tesisat','DMT-2024-0023','2024-04-20','2024-05-20',3150000,3150000,'TRY','odendi','HVAC ekipman tedariki'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Delta Mekanik Tesisat','DMT-2024-0041','2024-05-20','2024-06-20',2800000,0,'TRY','bekliyor','Yangin sondurme borulama'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ege Elektrik Malzemeleri','EEM-2024-0067','2024-04-10','2024-05-10',1920000,1920000,'TRY','odendi','Guc kabloları ve panolar'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ege Elektrik Malzemeleri','EEM-2024-0089','2024-05-10','2024-06-10',2340000,1170000,'TRY','kismi_odendi','Aydinlatma armaturleri'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Yapi Kimyasallari Ltd.','YKL-2024-0034','2024-04-05','2024-05-05',680000,680000,'TRY','odendi','Beton katki maddeleri'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Yapi Kimyasallari Ltd.','YKL-2024-0052','2024-05-05','2024-06-05',720000,0,'TRY','bekliyor','Su yalitim malzemeleri')
ON CONFLICT DO NOTHING;
