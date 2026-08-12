-- Migration 000036 — Hakediş metraj satırını bir saha tutanağına bağlama
-- ("tutanaklı imalat"): bir kalemin bu dönemki miktarının hangi tutanakla
-- belgelendiğini isteğe bağlı olarak işaretler.
ALTER TABLE progress_payment_items
    ADD COLUMN tutanak_id uuid NULL REFERENCES saha_tutanaklari(id) ON DELETE SET NULL;
