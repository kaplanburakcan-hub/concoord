-- 000068 ile aynı desen: Kiralama Sözleşmesi (Makine sayfası) ve Nakliye
-- İrsaliyesi (Transfer Talepleri) ayrı kategoriler kullanıyordu, bu yüzden
-- Dokümanlar sayfasındaki Sözleşme/İrsaliye filtrelerinde görünmüyorlardı.
-- Her iki sayfanın kendi listeleme sorgusu entity_type+entity_id+category
-- üçlüsüyle zaten tek bir kayda (ekipman/transfer talebi) özel filtrelendiği
-- için, kategori değerini değiştirmek kendi görünümlerini bozmaz — yalnızca
-- artık genel Dokümanlar taksonomisiyle eşleşir.
UPDATE documents SET doc_category = 'Contract' WHERE doc_category = 'KiralamaSozlesmesi';
UPDATE documents SET doc_category = 'Delivery' WHERE doc_category = 'NakliyeIrsaliyesi';
