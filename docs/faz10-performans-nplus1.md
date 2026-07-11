# Faz 10 — Performans: Index Analizi ve N+1 Taraması

Bu belge, Faz 10 sertleştirme kapsamında yapılan performans gözden geçirmesinin
kaydıdır. İki eksen: (1) veritabanı indeks kapsamı, (2) uygulama katmanı N+1
sorgu taraması.

## 1. Index Analizi

Faz 0–9 şeması sıcak yolları büyük ölçüde indeksler (proje bazlı listeler,
statü filtreleri, foreign key join'leri, `deleted_at IS NULL` kısmi indeksleri).
Analizde tespit edilen ve `000011_faz10_performance` ile kapatılan gerçek
boşluklar:

| Indeks | Gerekçe |
|---|---|
| `idx_payment_deductions_source` (kısmi) | İSG ceza → hakediş kesinti köprüsü geriye izleme (Plan §6) |
| `idx_ohs_penalties_applied` (kısmi) | Bir hakedişe uygulanmış cezaları bulma |
| `idx_pp_finalized` (kısmi) | EVM sıcak yolu — EV/AC yalnızca `Finalized` hakedişleri tarar |
| `idx_document_versions_file` | Versiyon → `files` join'i (indirme) |
| `idx_contracts_parent` (kısmi) | Sözleşme/zeyilname ağacı |

**Eklenmeyenler (bilinçli):** `role_permissions` rol join'i PRIMARY KEY
`(role_id, permission_id)` ile zaten kapsanır; `task_assignees` task_id ön-eki
PK ile kapsanır. Gereksiz indeks yazma maliyetini artırır — eklenmedi.

Çalışan sistemde doğrulama:
```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT ... FROM progress_payments WHERE project_id = $1 AND status = 'Finalized';
-- 000011 sonrası: Index Scan using idx_pp_finalized (Seq Scan DEĞİL)
```

## 2. N+1 Taraması

Handler'lar liste uçlarında satır-başına-sorgu (N+1) desenine karşı tarandı.
Genel durum sağlıklı: liste uçları `JOIN` / `array_agg` / tek sorgu +
`IN (...)` toplu getirme kullanıyor. İncelenen ana yollar:

| Uç | Durum | Not |
|---|---|---|
| `GET /projects/{id}/tasks` | ✔ Temiz | Atananlar ve yorumlar `array_agg`/toplu; kart başına sorgu yok |
| `GET /projects/{id}/payments` | ✔ Temiz | Kesintiler ve kalemler ayrı toplu sorgu, bellekte eşleştirilir |
| `GET /projects/{id}/documents` | ✔ Temiz | Son versiyon `LATERAL`/pencere ile tek sorguda |
| `GET /portfolio` | ✔ Temiz | Proje kartları toplu agregasyon; proje başına döngü sorgusu yok |
| `GET /projects/{id}/dashboard` | ✔ Temiz | EVM bileşenleri (PV/EV/AC) tek geçişte hesaplanır |
| `GET /projects/{id}/ohs/findings` | ✔ Temiz | Bulgu + foto doküman bağı tek sorgu |

**Sonuç:** Kod düzeyinde yeni N+1 düzeltmesi gerekmedi; performans kazanımı
indeks katmanında yoğunlaştı. Yük testi (`deploy/loadtest`) ile 100 eşzamanlı
kullanıcı altında p95 gecikme eşiği doğrulanır.

## 3. Bağlantı havuzu ve zaman aşımları
- `pgxpool` varsayılan havuzu kullanılır; VPS kaynak profiline göre
  `IPKS_DB_DSN` içinde `pool_max_conns` ayarlanabilir.
- HTTP sunucusunda `ReadHeaderTimeout` ayarlı (slowloris'e karşı).
- `readyz` DB ping'i 3 sn zaman aşımıyla sınırlı.
