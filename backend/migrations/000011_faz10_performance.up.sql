-- Faz 10 — Performans sertleştirmesi: index analizinde tespit edilen sıcak-yol
-- boşluklarını kapatır. Mevcut şema (Faz 0–9) zaten geniş ölçüde indekslidir;
-- burada yalnızca EAGER join / EVM toplama / otomasyon köprüsü sorgularında
-- kalan gerçek eksikler eklenir. IF NOT EXISTS ile yeniden çalıştırmaya güvenli.
--
-- N+1 taraması notları docs/faz10-performans-nplus1.md dosyasındadır; kod
-- düzeyi düzeltmeler (batch/JOIN) o belgede işaretlidir, bu migration yalnızca
-- indeks katmanını tamamlar.

-- NOT: role_permissions rol join'i için ayrı indeks EKLENMEDİ — PRIMARY KEY
-- (role_id, permission_id) zaten role_id ön-ekini kapsar (index analizi teyidi).

-- 1) İSG ceza → hakediş kesinti köprüsü (Plan §6, Faz 8): kaynak izlenebilirliği
--    (source_entity, source_id) ile geriye dönük sorgulanır.
CREATE INDEX IF NOT EXISTS idx_payment_deductions_source
    ON payment_deductions (source_entity, source_id)
    WHERE source_id IS NOT NULL;

-- 2) Bir hakedişe uygulanmış cezaları bulma (kesinti önerisi/uygulama akışı).
CREATE INDEX IF NOT EXISTS idx_ohs_penalties_applied
    ON ohs_penalties (applied_payment_id)
    WHERE applied_payment_id IS NOT NULL;

-- 3) EVM sıcak yolu (Faz 9): EV/AC hesabı yalnızca Finalized hakedişleri tarar.
--    Kısmi indeks büyük tabloda tarama alanını dramatik daraltır.
CREATE INDEX IF NOT EXISTS idx_pp_finalized
    ON progress_payments (project_id)
    WHERE status = 'Finalized' AND deleted_at IS NULL;

-- 4) Doküman versiyonu → files join'i (indirme/versiyon listesi).
CREATE INDEX IF NOT EXISTS idx_document_versions_file
    ON document_versions (file_id);

-- 5) Sözleşme zeyilname ağacı (parent_contract_id ile çocuk sözleşmeler).
CREATE INDEX IF NOT EXISTS idx_contracts_parent
    ON contracts (parent_contract_id)
    WHERE parent_contract_id IS NOT NULL;

-- İstatistikleri planlayıcı için tazele (yeni indeksler hemen kullanılsın).
ANALYZE;
