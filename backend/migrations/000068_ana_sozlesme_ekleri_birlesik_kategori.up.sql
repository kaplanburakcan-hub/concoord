-- Ana Sözleşme Ekleri (ContractAttachments) yüklemeleri "AnaSozlesmeEki"
-- ayrı kategorisiyle kaydediliyordu; Dokümanlar sayfasının "Sözleşme"
-- filtresi ise doc_category='Contract' arıyor — bu yüzden bir projeye Ana
-- Sözleşme eki eklense bile Dokümanlar > Sözleşme'de hiç görünmüyordu.
-- entity_type='main_contract' zaten kayıtları kendi bağlamında ayırt
-- ettiğinden (ContractAttachments kendi listesini category'den bağımsız,
-- yalnızca entity_type+entity_id ile çeker), ayrı bir kategoriye gerek yok.
UPDATE documents SET doc_category = 'Contract' WHERE doc_category = 'AnaSozlesmeEki';
