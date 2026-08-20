-- İş kazası kayıtlarına inceleme durumu: İnceleniyor/Kapandı. ohs_findings'in
-- Open/InProgress/Closed yaşam döngüsünden farklı, daha basit tek geçişli
-- (Investigating -> Closed) bir durum — kaza kaydı zaten fiilen gerçekleşmiş
-- bir olayın tarih kaydı, tekrar açılan bir "iş" değil.
ALTER TABLE ohs_accidents
    ADD COLUMN status text NOT NULL DEFAULT 'Investigating'
        CHECK (status IN ('Investigating','Closed')),
    ADD COLUMN closed_by uuid REFERENCES users (id),
    ADD COLUMN closed_at timestamptz,
    ADD COLUMN close_note text;
