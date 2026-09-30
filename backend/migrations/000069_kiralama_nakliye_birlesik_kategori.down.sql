-- Geri alma: entity_type ile ayırt edilebilir (bu kategoriler yalnızca bu
-- entity_type'larla kullanılıyordu).
UPDATE documents SET doc_category = 'KiralamaSozlesmesi'
    WHERE doc_category = 'Contract' AND entity_type = 'company_equipment';
UPDATE documents SET doc_category = 'NakliyeIrsaliyesi'
    WHERE doc_category = 'Delivery' AND entity_type = 'equipment_transfer_requests';
