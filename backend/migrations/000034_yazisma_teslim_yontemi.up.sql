-- Migration 000034 — Yazışmalarda teslim yöntemi (e-posta / fiziksel)
--
-- Gelen/giden evrakın mail yoluyla mı yoksa imzalı/barkodlu kağıt olarak mı
-- teslim edildiğini ayırt etmek için tek bir alan. "fiziksel" hem ıslak
-- imzalı hem barkodlu kağıt teslimatını kapsar — kullanıcı ikon düzeyinde
-- tek bir ayrım istiyor (damgalı kağıt vs. e-posta), alt tür değil.

ALTER TABLE correspondences
    ADD COLUMN teslim_yontemi text NOT NULL DEFAULT 'eposta'
        CHECK (teslim_yontemi IN ('eposta','fiziksel'));
