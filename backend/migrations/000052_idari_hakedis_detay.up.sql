-- İdari Hakedişler: gerçek hakediş raporu düzenine göre detaylandırma.
-- tutar kolonu artık DOĞRUDAN girilmiyor — sözleşme fiyatları + fiyat
-- farkı + önceki hakediş + KDV + kesintiler üzerinden backend'de
-- hesaplanıp yazılıyor (bkz. internal/idarihakedis/handler.go calc()).

ALTER TABLE idari_hakedisler
    ADD COLUMN hakedis_tarihi date,
    ADD COLUMN sozlesme_fiyatlari_tutari numeric(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN fiyat_farki_tutari numeric(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN onceki_hakedis_toplami numeric(20,2) NOT NULL DEFAULT 0,
    -- [{ad, tutar}] — rapordaki a-i kesinti/mahsup kalemleri + serbest ek satırlar.
    ADD COLUMN kesintiler jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Hakediş belgesi (imzalı kapak sayfası ya da komple hakediş) — mevcut
-- IdariHakedisFatura kategorisinden ayrı, aynı entity_type üzerinde
-- (entity_id = idari_hakedisler.id) farklı bir kategori.
ALTER TABLE documents DROP CONSTRAINT documents_doc_category_check;
ALTER TABLE documents ADD CONSTRAINT documents_doc_category_check
    CHECK (doc_category IN ('Contract','Addendum','Submittal','Drawing','Delivery','OHS',
        'SahaTutanagi','SahaFotografi','ImalatFotografi','DenetimFotografi',
        'IdariHakedisFatura','IdariHakedisBelgesi','ProjeGorseli','NakliyeIrsaliyesi',
        'KiralamaSozlesmesi','Other'));
