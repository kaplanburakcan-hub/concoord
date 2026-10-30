-- Faz 3.1 / E8-E11 — yaklaşan teslim bildirimi için tekilleştirme işareti.
-- Bir sipariş, beklenen teslim tarihi değişmedikçe yalnız bir kez uyarılır.
ALTER TABLE purchase_orders
    ADD COLUMN approaching_notified_at timestamptz NULL;
