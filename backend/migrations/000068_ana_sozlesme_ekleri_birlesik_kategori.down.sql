-- Geri alma: hangi "Contract" kayıtlarının eskiden "AnaSozlesmeEki" olduğunu
-- ayırt etmenin güvenli yolu entity_type='main_contract' olanlardır (genel
-- "Contract" kategorisi başka entity_type'larla da kullanılıyor olabilir).
UPDATE documents SET doc_category = 'AnaSozlesmeEki'
    WHERE doc_category = 'Contract' AND entity_type = 'main_contract';
