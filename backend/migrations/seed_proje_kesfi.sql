-- Seed: Proje Keşfi Veri Seti
-- 5 aktif proje için 7 kategori × gerçekçi imalat kalemleri
-- Çalıştırma: psql <dsn> -f seed_proje_kesfi.sql
-- NOT: Windows'ta psql'in istemci kodlama ayarı konsol code page'ine göre
-- WIN1252/CP850 olabilir; bu satır olmadan Türkçe karakterler (ş,ç,ğ,ı,ö,ü)
-- çift kodlanıp DB'ye bozuk (mojibake) yazılır. Bkz. project_survey_items
-- düzeltmesi (2026-08-16).
SET client_encoding = 'UTF8';

-- ─── DEMO-04 | Nilüfer Şehir Hastanesi Kompleksi ───────────────────────────
-- Proje: 06802be9-d6e4-4bc8-abb4-6aaa6bfc840f  (~320M TRY götürü sözleşme)

INSERT INTO project_survey_items (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, sira) VALUES
-- Betonarme
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.014','C30/37 Beton – Temel raft ve kazık başlığı',      'm³',  4200, 2850.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.016','C30/37 Beton – Kolon ve perde',                   'm³',  6800, 3100.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.018','C30/37 Beton – Kirişli döşeme (h=28 cm)',         'm³',  9200, 2950.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.101','B500C Nervürlü donatı çeliği',                    'ton',  950, 28500.00,'TRY', 40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.201','Çelik kalıp – perde ve kolon',                    'm²',38000,  380.00,'TRY', 50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Betonarme','23.205','Alüminyum kolon kalıbı kiralama',                 'm²', 8500,  220.00,'TRY', 60),
-- Mimari
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.011','Ytong gazbeton duvar blok (d=20 cm)',                'm²',42000,  680.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.021','Seramik yer kaplaması 60×60 (hastane serisi)',       'm²',18000,  850.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.031','Asma tavan – alçıpan kompozit sistem',               'm²',22000,  520.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.041','İç kapı – çelik sandviç (90×210)',                  'adet', 1850, 4200.00,'TRY', 40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.051','Mermer merdiven basamak ve sahanlık',                'm²',  820, 1850.00,'TRY', 50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mimari','27.061','İç mekan boyası (silikon bazlı)',                    'm²',78000,   95.00,'TRY', 60),
-- Cephe
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Cephe','28.011','Isı yalıtımlı giydirme cephe sistemi (unitize)',     'm²',12500, 4800.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Cephe','28.021','Alüminyum kompozit panel cephe kaplama',             'm²', 3800, 1250.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Cephe','28.031','Dış cephe mantolama (EPS 10 cm + sıva)',             'm²', 6200,  580.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Cephe','28.041','Alüminyum doğrama – dış kapı ve pencere',            'm²', 2100, 3200.00,'TRY', 40),
-- Çatı
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Çatı','29.011','Çatı çelik ana taşıyıcı ve aşık sistemi',            'ton',   85,52000.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Çatı','29.021','Sandviç panel çatı kaplama (10 cm PUR)',              'm²', 8400, 1450.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Çatı','29.031','Su yalıtımı – PVC membran 2 kat',                    'm²', 8400,  480.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Çatı','29.041','Çatı ışıklığı alüminyum piramit sistem',              'adet',  18,28000.00,'TRY', 40),
-- Mekanik
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.011','Merkezi VRF klima sistemi (full inverter)',         'set',   12,920000.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.021','Mekanik havalandırma tesisatı (AHU+kanallar)',      'm²',28000,  850.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.031','Sıhhi tesisat – tüm kat borulama ve armatürler',  'adet',1200, 3800.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.041','Yangın söndürme sistemi (sprinkler)',               'm²',28000,  280.00,'TRY', 40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.051','Tıbbi gaz tesisatı (O₂, N₂O, vakum, basınçlı hava)','oda',  680,4500.00,'TRY', 50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Mekanik','30.061','Isıtma tesisatı – doğalgaz kazanı ve paneller',    'm²',28000,  420.00,'TRY', 60),
-- Elektrik
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.011','Orta gerilim (OG) hücresi ve trafo merkezi (3×1250 kVA)','set',1,1850000.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.021','AG pano – kat dağıtım tabloları',                 'adet',  48, 18500.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.031','Elektrik tesisatı (kablo + kanallar + montaj)',    'm²',28000,  950.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.041','Aydınlatma – LED armatür ve montaj',              'adet',8500,  750.00,'TRY', 40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.051','Jeneratör (2×1000 kVA, otomatik transfer)',       'adet',   2,2800000.00,'TRY', 50),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Elektrik','31.061','Zayıf akım – data/BMS/KNX/CCTV/access control',  'm²',28000,  380.00,'TRY', 60),
-- Peyzaj
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','32.011','Sert zemin – granit karo yol ve meydan',            'm²', 4800,  680.00,'TRY', 10),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','32.021','Yeşil alan – çim ekimi ve sulaması',                'm²',12000,  180.00,'TRY', 20),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','32.031','Ağaç – yarı büyük boy (4-6 m) dikim ve bakım',    'adet',  280, 4800.00,'TRY', 30),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','32.041','Dış aydınlatma direği ve armatür (LED)',            'adet',  185, 8500.00,'TRY', 40),
('06802be9-d6e4-4bc8-abb4-6aaa6bfc840f','Peyzaj','32.051','Otopark zemin çizgileri ve levhalar',               'm²', 3200,   95.00,'TRY', 50);

-- ─── DEMO-01 | Bahçeşehir 480 Konut ve Sosyal Tesis ───────────────────────
-- Proje: 853447da-3cb2-4c83-b5a7-331d22329547  (~480 konut, çok katlı)

INSERT INTO project_survey_items (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, sira) VALUES
-- Betonarme
('853447da-3cb2-4c83-b5a7-331d22329547','Betonarme','23.014','C25/30 Beton – sürekli temel ve radye',            'm³',  5800, 2700.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Betonarme','23.016','C30/37 Beton – perde ve kolon (bodrum+kat)',       'm³',  8400, 3050.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Betonarme','23.018','C30/37 Beton – asmolen döşeme',                   'm³', 11500, 2800.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Betonarme','23.101','B420C Nervürlü donatı çeliği',                    'ton', 1250,27800.00,'TRY', 40),
('853447da-3cb2-4c83-b5a7-331d22329547','Betonarme','23.201','Ahşap ve çelik kalıp sistemi',                    'm²',55000,  320.00,'TRY', 50),
-- Mimari
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.011','Tuğla bölme duvar (d=13,5 cm)',                      'm²',68000,  420.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.021','Seramik yer kaplaması 60×60 (konut serisi)',          'm²',62000,  520.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.022','Parke – laminat 8 mm (yatak odaları)',               'm²',28000,  380.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.031','Alçı sıva ve silika – iç duvar',                    'm²',95000,  185.00,'TRY', 40),
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.041','Ahşap iç kapı seti (90×210) + montaj',              'adet', 3840, 2800.00,'TRY', 50),
('853447da-3cb2-4c83-b5a7-331d22329547','Mimari','27.061','İç mekan boyası – plastik boya 2 kat',               'm²',138000,   75.00,'TRY', 60),
-- Cephe
('853447da-3cb2-4c83-b5a7-331d22329547','Cephe','28.031','Dış cephe mantolama (EPS 8 cm + ince sıva)',          'm²',22000,  520.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Cephe','28.021','Alüminyum kompozit panel – balkon ve cephe aksanı',   'm²', 4200, 1100.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Cephe','28.041','PVC sürgülü pencere sistemi (ısıcam + ısı köprü)',   'm²', 8500, 1850.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Cephe','28.051','Alüminyum balkon korkuluğu + cam dolgu',              'm',  4800,  680.00,'TRY', 40),
-- Çatı
('853447da-3cb2-4c83-b5a7-331d22329547','Çatı','29.021','Çatı – bitümlü çift kat membran + XPS ısı yalıtımı',  'm²', 6800,  580.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Çatı','29.022','Teras çatı – yeşil çatı kaplama sistemi',             'm²', 1200,  950.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Çatı','29.031','Oluk, iniş borusu ve saçak detay (alüminyum)',        'm',  2800,  320.00,'TRY', 30),
-- Mekanik
('853447da-3cb2-4c83-b5a7-331d22329547','Mekanik','30.011','Merkezi sistem çiller + fan-coil (konut bloğu)',   'adet', 480, 38000.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Mekanik','30.021','Doğalgaz kombi tesisatı + bacalar',               'daire', 480, 12500.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Mekanik','30.031','Sıhhi tesisat – daire montaj takımı',             'daire', 480,  8200.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Mekanik','30.041','Yangın sprinkler sistemi',                         'm²',32000,  250.00,'TRY', 40),
-- Elektrik
('853447da-3cb2-4c83-b5a7-331d22329547','Elektrik','31.011','Trafo merkezi + OG hücresi (2×800 kVA)',          'set',    1,1250000.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Elektrik','31.021','Elektrik tesisatı – daire paket',                'daire',  480, 14500.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Elektrik','31.031','Ortak alan aydınlatma ve acil aydınlatma',        'kat',    68,  8500.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Elektrik','31.041','Asansör (8 durak, 8 kişilik) + montaj',          'adet',   12,420000.00,'TRY', 40),
-- Peyzaj
('853447da-3cb2-4c83-b5a7-331d22329547','Peyzaj','32.011','Sert zemin – beton kilit taşı (yol ve otopark)',    'm²', 8500,  280.00,'TRY', 10),
('853447da-3cb2-4c83-b5a7-331d22329547','Peyzaj','32.021','Yeşil alan – çim, çalı ve mevsimlik çiçek',        'm²',18000,  150.00,'TRY', 20),
('853447da-3cb2-4c83-b5a7-331d22329547','Peyzaj','32.031','Çocuk oyun parkı donanımı + zemin kauçuk',         'set',    4, 95000.00,'TRY', 30),
('853447da-3cb2-4c83-b5a7-331d22329547','Peyzaj','32.041','Dış aydınlatma ve peyzaj aydınlatması',            'adet',  220,  6800.00,'TRY', 40);

-- ─── DEMO-02 | Polatlı–Sivrihisar Otoyol ve Köprü Yapımı ─────────────────
-- Proje: 61992266-4b84-422c-8d66-8a0d16ebb778  (altyapı / yol yapımı)

INSERT INTO project_survey_items (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, sira) VALUES
-- Betonarme
('61992266-4b84-422c-8d66-8a0d16ebb778','Betonarme','23.014','C35/45 Köprü kirişleri – öngerilmeli (precast)',  'm³',  3800, 5200.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Betonarme','23.016','C30/37 Köprü başabaşı, tabliye levhası',          'm³',  2100, 3800.00,'TRY', 20),
('61992266-4b84-422c-8d66-8a0d16ebb778','Betonarme','23.017','Köprü kazık temeli (fore kazık d=120 cm)',       'adet',  240,38000.00,'TRY', 30),
('61992266-4b84-422c-8d66-8a0d16ebb778','Betonarme','23.101','B500C Köprü donatısı',                            'ton',  580,29500.00,'TRY', 40),
-- Mimari (Altyapı/Üstyapı olarak kullanılıyor)
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.001','Yol kazısı – sıkışmış zemin (buldozer+kamyon)',      'm³',850000,   42.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.002','Dolgu – seçilmiş malzeme, sıkıştırmalı',            'm³',620000,   38.00,'TRY', 20),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.011','Temel tabakası – kırılmış granüler malzeme (h=30 cm)','m²',185000,  125.00,'TRY', 30),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.021','Bitümlü temel tabakası (BTB) 6 cm',                  'm²',165000,  285.00,'TRY', 40),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.022','Bitümlü binder tabakası (BB) 5 cm',                  'm²',165000,  260.00,'TRY', 50),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mimari','21.023','Aşınma tabakası (SMA) 4 cm',                        'm²',165000,  310.00,'TRY', 60),
-- Cephe (Köprü kaplama / koruma)
('61992266-4b84-422c-8d66-8a0d16ebb778','Cephe','28.090','Köprü korkuluğu – galvaniz çelik bariyer',           'm',  1850,  680.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Cephe','28.091','Köprü yüzeyi – epoksi kaplama ve anti-karbon',       'm²', 8200,  320.00,'TRY', 20),
-- Çatı (Köprü geçici çalışmalar / BSK)
('61992266-4b84-422c-8d66-8a0d16ebb778','Çatı','29.090','Köprü su yalıtımı – modifiye bitüm membran',          'm²', 4800,  420.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Çatı','29.091','Köprü BSK örtüsü – 4 cm',                            'm²', 4800,  380.00,'TRY', 20),
-- Mekanik (Altyapı)
('61992266-4b84-422c-8d66-8a0d16ebb778','Mekanik','30.091','Yol drenaj sistemi – beton menfez ve kanallar',    'm',  8500,  850.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mekanik','30.092','Yağmur suyu ızgara ve baca (D400)',               'adet',  380, 2800.00,'TRY', 20),
('61992266-4b84-422c-8d66-8a0d16ebb778','Mekanik','30.093','Geçiş altı viyadük menfezi ve borulama',           'm³',  650, 3500.00,'TRY', 30),
-- Elektrik (Aydınlatma)
('61992266-4b84-422c-8d66-8a0d16ebb778','Elektrik','31.091','Yol aydınlatma direği (10 m, galvaniz) + LED',   'adet',  420,18500.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Elektrik','31.092','Enerji nakil hattı + pano – yol aydınlatması',   'km',    28, 85000.00,'TRY', 20),
-- Peyzaj
('61992266-4b84-422c-8d66-8a0d16ebb778','Peyzaj','32.091','Yol kenarı çevre düzenlemesi (çim + yeşil bantlar)','m²',42000,   85.00,'TRY', 10),
('61992266-4b84-422c-8d66-8a0d16ebb778','Peyzaj','32.092','Yol güvenlik bariyeri – beton W-tipi',             'm',  18500,  320.00,'TRY', 20),
('61992266-4b84-422c-8d66-8a0d16ebb778','Peyzaj','32.093','Yol işaretleme – termoplastik boya ve trafik işaretleri','m²', 4800,  185.00,'TRY', 30);

-- ─── DEMO-03 | Kent Plaza AVM ve Karma Kullanım Yapısı ────────────────────
-- Proje: cb5289a3-a760-489a-af63-28ecdb017b46  (AVM / ticari)

INSERT INTO project_survey_items (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, sira) VALUES
-- Betonarme
('cb5289a3-a760-489a-af63-28ecdb017b46','Betonarme','23.014','C32/40 Beton – derin radye (h=150 cm)',            'm³',  6200, 3100.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Betonarme','23.016','C35/45 Beton – mega kolon ve çekirdek perdeler',  'm³',  9800, 3400.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Betonarme','23.018','C30/37 Beton – kirişli nervürlü döşeme',          'm³', 14200, 3000.00,'TRY', 30),
('cb5289a3-a760-489a-af63-28ecdb017b46','Betonarme','23.101','B500C Donatı – yapısal çelik',                    'ton', 1680,28800.00,'TRY', 40),
-- Mimari
('cb5289a3-a760-489a-af63-28ecdb017b46','Mimari','27.021','Doğal taş zemin kaplama – granit (60×60 cilalanmış)','m²',32000, 1250.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mimari','27.031','Asma tavan – alüminyum tip panel (600×600)',         'm²',28000,  650.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mimari','27.041','Otomatik kayar kapı (120×240, algılayıcılı)',        'adet',  142,22000.00,'TRY', 30),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mimari','27.051','Mağaza cephe bölücü duvar sistemi (cam+çelik)',      'm²', 6800, 2800.00,'TRY', 40),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mimari','27.061','Ortak alan boyası ve dekoratif kaplama',             'm²',42000,  220.00,'TRY', 50),
-- Cephe
('cb5289a3-a760-489a-af63-28ecdb017b46','Cephe','28.011','Structral glazing cam cephe – ısı yalıtımlı çift cam','m²',18500, 6800.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Cephe','28.021','Alüminyum kazıklı cephe sistemi + delikli panel',    'm²', 4200, 2200.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Cephe','28.051','Çelik cam köprü ve pasaj köprüsü (4 adet)',          'adet',   4,850000.00,'TRY', 30),
-- Çatı
('cb5289a3-a760-489a-af63-28ecdb017b46','Çatı','29.011','Çatı çelik ana taşıyıcı + uzay çatı sistemi',         'ton',  320,58000.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Çatı','29.021','Çatı cam aydınlık – üçgen prizmatik sistem',          'm²', 3200, 8500.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Çatı','29.031','Su yalıtımı PVC membran + ısı yalıtım XPS',           'm²',12800,  620.00,'TRY', 30),
-- Mekanik
('cb5289a3-a760-489a-af63-28ecdb017b46','Mekanik','30.011','Merkezi chiller sistemi (2×2000 kW soğutma)',      'set',    1,8500000.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mekanik','30.021','Havalandırma tesisatı AHU + kanal sistemi',        'm²',68000,  780.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mekanik','30.031','Sıhhi tesisat – tuvalet paket+borulama',           'wc',   380, 8500.00,'TRY', 30),
('cb5289a3-a760-489a-af63-28ecdb017b46','Mekanik','30.041','Yangın söndürme (sprinkler + hidrant)',             'm²',68000,  280.00,'TRY', 40),
-- Elektrik
('cb5289a3-a760-489a-af63-28ecdb017b46','Elektrik','31.011','OG merkezi + trafo (3×2000 kVA) + jeneratör',    'set',    1,4800000.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Elektrik','31.021','Elektrik tesisatı – güç ve aydınlatma',           'm²',68000,  850.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Elektrik','31.031','Asansör + yürüyen merdiven paketi',               'adet',  48, 680000.00,'TRY', 30),
('cb5289a3-a760-489a-af63-28ecdb017b46','Elektrik','31.061','Zayıf akım – BMS/yangın alarm/CCTV/BAS/IP/AV',    'm²',68000,  420.00,'TRY', 40),
-- Peyzaj
('cb5289a3-a760-489a-af63-28ecdb017b46','Peyzaj','32.011','Dış meydan – granit taş kaplama + aydınlatma',     'm²', 6800, 1100.00,'TRY', 10),
('cb5289a3-a760-489a-af63-28ecdb017b46','Peyzaj','32.021','Çatı bahçesi yeşil alan sistemi',                   'm²', 2800, 1200.00,'TRY', 20),
('cb5289a3-a760-489a-af63-28ecdb017b46','Peyzaj','32.041','Dış aydınlatma ve dekoratif peyzaj',               'adet',  380,  9500.00,'TRY', 30);

-- ─── CEN-01 | Veri Merkezi Yapım İşi ─────────────────────────────────────
-- Proje: 75f4d587-8929-4e7a-b222-f12d6880986d  (veri merkezi / yüksek teknoloji)

INSERT INTO project_survey_items (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, sira) VALUES
-- Betonarme
('75f4d587-8929-4e7a-b222-f12d6880986d','Betonarme','23.014','C35/45 Beton – titreşim yalıtımlı temel plak',  'm³', 2800, 3500.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Betonarme','23.016','C30/37 Beton – EMP korumalı perde ve kolon',    'm³', 3200, 3800.00,'TRY', 20),
('75f4d587-8929-4e7a-b222-f12d6880986d','Betonarme','23.101','B500C Donatı çeliği + Faraday kafesi bağlantıları','ton', 420,30000.00,'TRY', 30),
-- Mimari
('75f4d587-8929-4e7a-b222-f12d6880986d','Mimari','27.021','Antistatik epoksi zemin kaplama (kalın katmanlı)',   'm²', 8500,  980.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mimari','27.022','Yükseltilmiş döşeme sistemi (raised floor, h=60 cm)','m²', 6200, 2800.00,'TRY', 20),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mimari','27.031','Akustik tavan paneli – yangın yalıtımlı',          'm²', 8500,  680.00,'TRY', 30),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mimari','27.041','Çelik güvenlik kapısı (EMP korumalı, RF gasket)',  'adet',  85,28000.00,'TRY', 40),
-- Cephe
('75f4d587-8929-4e7a-b222-f12d6880986d','Cephe','28.021','Alüminyum sandwich panel – EMP perdeli cephe',      'm²', 6800, 3200.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Cephe','28.041','Güvenlikli cam cephe (kurşun geçirmez, 6A/6)',       'm²',  820,18000.00,'TRY', 20),
-- Çatı
('75f4d587-8929-4e7a-b222-f12d6880986d','Çatı','29.031','Çatı su yalıtımı + ısı yalıtımı (XPS 15 cm)',        'm²', 4200,  850.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Çatı','29.022','UPS soğutucu üniteleri çatı platformu + çelik taşıyıcı','ton',  45,65000.00,'TRY', 20),
-- Mekanik
('75f4d587-8929-4e7a-b222-f12d6880986d','Mekanik','30.011','Hassas soğutma – in-row CRAC ünitesi (60 kW)',    'adet',  48,420000.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mekanik','30.012','Chiller soğutma kulesi (2×1200 kW + N+1)',        'set',    1,18500000.00,'TRY', 20),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mekanik','30.041','Gaz söndürme sistemi – Novec 1230 (bilgi işlem odaları)','oda', 12,185000.00,'TRY', 30),
('75f4d587-8929-4e7a-b222-f12d6880986d','Mekanik','30.042','Sıvı soğutma – pipe & pump (CDU) sistemi',        'kW', 5000,  2800.00,'TRY', 40),
-- Elektrik
('75f4d587-8929-4e7a-b222-f12d6880986d','Elektrik','31.011','2N güç mimarisi – OG+trafo (2×3000 kVA) + jeneratör (2×2500 kVA)','set',1,38000000.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Elektrik','31.012','UPS sistemi – modüler (400 kVA × 12 modül)',     'adet',  12,1850000.00,'TRY', 20),
('75f4d587-8929-4e7a-b222-f12d6880986d','Elektrik','31.021','Güç dağıtım – PDU rack ve busduct sistemi',      'kW',12000,  1200.00,'TRY', 30),
('75f4d587-8929-4e7a-b222-f12d6880986d','Elektrik','31.061','Zayıf akım – BMS/EPMS/DCIM/çevre izleme/CCTV/erişim','m²',9500,  1850.00,'TRY', 40),
-- Peyzaj
('75f4d587-8929-4e7a-b222-f12d6880986d','Peyzaj','32.011','Güvenlik çit sistemi – çelik + dikenli tel + CCTV','m',   850,  2800.00,'TRY', 10),
('75f4d587-8929-4e7a-b222-f12d6880986d','Peyzaj','32.021','Bariyer & araç durdurucu (bollard) sistemi',       'adet',  28, 45000.00,'TRY', 20),
('75f4d587-8929-4e7a-b222-f12d6880986d','Peyzaj','32.031','Saha düzenlemesi – betonlamalı yol ve otopark',    'm²', 3800,   580.00,'TRY', 30);
