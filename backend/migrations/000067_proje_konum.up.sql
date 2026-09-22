-- Proje künyesi: yapısal konum alanları (Ülke/İl/İlçe + opsiyonel koordinat).
-- Konum/Vaziyet Planı Görseli'nin yerini alan otomatik harita görüntüsü
-- "documents" tablosuna (doc_category='KonumHaritasi') kaydedilir; ayrı bir
-- kolon gerekmez.
ALTER TABLE projects
  ADD COLUMN ulke text,
  ADD COLUMN il text,
  ADD COLUMN ilce text,
  ADD COLUMN enlem numeric(9, 6),
  ADD COLUMN boylam numeric(9, 6);

-- Bulgu: documents.doc_category CHECK kısıtı "KonumGorseli"yi (migration
-- 000055'te yalnızca Go docCategories map'ine eklenmiş) hiçbir zaman
-- içermemişti — o yükleme kutusu kullanılsaydı DB seviyesinde
-- reddedilecekti. Burada hem yeni "KonumHaritasi" kategorisi ekleniyor
-- hem de bu eksik tutarlı hale getiriliyor (geriye dönük, artık
-- kullanılmasa da).
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','IdariHakedisBelgesi','ProjeGorseli','KonumGorseli','KonumHaritasi',
        'NakliyeIrsaliyesi','KiralamaSozlesmesi','AnaSozlesmeEki','Other'));
