SET client_encoding = 'UTF8';

-- Proje: DEMO-04 Nilüfer Şehir Hastanesi Kompleksi
-- project_id: 06802be9-d6e4-4bc8-abb4-6aaa6bfc840f

-- Personel
INSERT INTO project_personnel (project_id, ad_soyad, gorev, firma, is_aktif, sira) VALUES
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
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Baris Cinar','Isci','Ana Yuklenici',false,150);

-- Tedarikçi Ekstreler
INSERT INTO supplier_statements
    (project_id, tedarikci_adi, ekstre_no, ekstre_tarihi, vade_tarihi,
     toplam_tutar, odenen_tutar, para_birimi, odeme_durumu, aciklama) VALUES
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0042','2024-03-01','2024-04-01',4250000,4250000,'TRY','odendi','Mart inşaat demiri teslimatı'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0071','2024-04-01','2024-05-01',5180000,5180000,'TRY','odendi','Nisan inşaat demiri teslimatı'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Turk Demir Celik A.S.','TDC-2024-0098','2024-05-01','2024-06-01',4920000,2500000,'TRY','kismi_odendi','Mayıs teslimatı - kısmi ödeme'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0115','2024-03-15','2024-04-15',8600000,8600000,'TRY','odendi','Mart hazır beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0143','2024-04-15','2024-05-15',9250000,9250000,'TRY','odendi','Nisan hazır beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Anadolu Beton Sanayi','ABS-2024-0178','2024-05-15','2024-06-15',7800000,0,'TRY','bekliyor','Mayıs hazır beton'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Delta Mekanik Tesisat','DMT-2024-0023','2024-04-20','2024-05-20',3150000,3150000,'TRY','odendi','HVAC ekipman tedariki'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Delta Mekanik Tesisat','DMT-2024-0041','2024-05-20','2024-06-20',2800000,0,'TRY','bekliyor','Yangın söndürme borulama'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ege Elektrik Malzemeleri','EEM-2024-0067','2024-04-10','2024-05-10',1920000,1920000,'TRY','odendi','Güç kabloları ve panolar'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Ege Elektrik Malzemeleri','EEM-2024-0089','2024-05-10','2024-06-10',2340000,1170000,'TRY','kismi_odendi','Aydınlatma armatürleri'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Yapi Kimyasallari Ltd.','YKL-2024-0034','2024-04-05','2024-05-05',680000,680000,'TRY','odendi','Beton katkı maddeleri'),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Yapi Kimyasallari Ltd.','YKL-2024-0052','2024-05-05','2024-06-05',720000,0,'TRY','bekliyor','Su yalıtım malzemeleri');
