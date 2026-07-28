-- Seed: Tasarim ve Projeler (project_design_docs)
-- Proje: DEMO-04  Nilüfer Şehir Hastanesi Kompleksi
-- project_id: 06802be9-d6e4-4bc8-abb4-6aaa6bfc840f

SET client_encoding = 'UTF8';

INSERT INTO project_design_docs
    (project_id, disiplin, poz_no, baslik, rev_no, tarih, durum, aciklama, sira)
VALUES
-- Mimari
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.000','Vaziyet ve Yerlesim Plani','C','2024-03-15','onaylı','Onaylandı, inşaata esas',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.101','Bodrum Kat Mimari Plani','D','2024-04-20','onaylı','Teknik Detaylar Tamamlandi',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.102','Zemin Kat Mimari Plani','E','2024-05-10','onaylı','Son Revizyon Onaylandi',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.103','1-4 Normal Kat Mimari Plani','B','2024-06-05','incelemede','Islah notlari bekleniyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.104','5-8 Normal Kat Mimari Plani','A','2024-07-01','revizyon_gerekli','Koridor genisligi revize edilecek',50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.201','Cephe Detaylari','B','2024-05-20','onaylı','Cephe sistemi onaylandi',60),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','A.301','Merdiven ve Asansor Detaylari','A','2024-06-15','incelemede','Bakanlik Engelli Erisilebilirligi incelemesinde',70),
-- Statik
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.001','Zemin Etud ve Sondaj Raporu','1','2024-01-10','onaylı','TMMOB onaylı zemin raporu',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.101','Temel Sistemi Plani (Kaz. Burgu+Raft)','C','2024-03-20','onaylı','Kazik hesaplari revize edildi',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.201','Bodrum Perde ve Kolon Yerlesimi','B','2024-04-15','onaylı','Betonarme hesaplariyla uyumlu',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.202','Zemin-8 Kat Kiriş ve Kolon Aplikasyonu','B','2024-05-25','incelemede','Hesap raporu teslim edildi, onay bekleniyor',40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Statik','S.301','Çelik Çatı Makasları ve Bağlantı Detaylari','A','2024-07-10','taslak','Tasarım devam ediyor',50),
-- Mekanik
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.001','Sıhhi Tesisat Şematik Planı','B','2024-04-05','onaylı','Tesisat ve boru güzergahlari onaylandi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.101','Yangın Söndürme Sistemi Planı','B','2024-04-25','onaylı','Itfaiye onayı alındı',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.201','HVAC Hava Kanallari Planı','A','2024-06-01','incelemede','Enerji kimlik belgesi hazirlanıyor',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','M.301','Tıbbi Gaz Sistemi (O2, N2O, Vakum)','A','2024-06-20','taslak','Uzman firma tasarımı bekleniyor',40),
-- Elektrik
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.001','AG-YG Güc Dagitim Sematigi','C','2024-03-10','onaylı','Trafo ve jeneratör gücü netlestirildi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.101','Aydinlatma ve Acil Aydinlatma Plani','B','2024-05-05','onaylı','EN 12464 normuna uygun',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.201','Yapisal Kablo Altyapisi (SCS)','A','2024-06-10','incelemede','Cat6A tercih onayı bekleniyor',30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','E.301','CCTV ve Erişim Kontrol Sistemi','0','2024-07-15','taslak','Güvenlik firması ile koordinasyon surüyor',40),
-- Iç Mimari
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.001','Zemin Kat Resepsiyon ve Bekleme Alanı','B','2024-05-15','onaylı','Malzeme listesi kesinlestirildi',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.101','Cerrahi Blok İç Mekan Tasarimi','A','2024-06-25','incelemede','Hijyen sartnamesi uyum kontrolü yapılıyor',20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','İç Mimari','IC.201','Hasta Odaları Detay Planı','0',NULL,'taslak','Konsept aşamasında',30),
-- Peyzaj
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','P.001','Dış Mekan Düzenleme Planı','A','2024-05-30','incelemede','Belediye onayı bekleniyor',10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','P.101','Otopark ve Yaya Aksları','A','2024-06-08','taslak','Trafik yoğunluk raporu alındı',20);
