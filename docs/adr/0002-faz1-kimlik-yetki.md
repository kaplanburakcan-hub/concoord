# ADR-0002 — Faz 1 Kimlik ve Yetki Kararları

**Tarih:** 06.07.2026 · **Durum:** Kabul edildi

## Bağlam
Faz 1, JWT auth + RBAC (kullanıcı bazlı override) + Admin Paneli v1'i teslim eder.
Plan §4 karar önceliğini zorunlu kılar: **DENY > GRANT > rol varsayılanı**.

## Kararlar

1. **İzin/rol verisi tek kaynak: `internal/rbac` (Go).** `AllPermissions`, `Roles`
   ve `RoleDefaults` hem seed'i (DB'ye yazar) hem de otomatik yetki test matrisini
   besler. Böylece "seed ne yazdıysa test onu doğrular" kod düzeyinde garanti edilir.
   Yeni izin = tek satır veri; kod değişmez (Plan ilke #2).

2. **Access token stateless (HS256 JWT), refresh token stateful.** JWT stdlib ile
   üretilir (ek imza bağımlılığı yok). Refresh token opak rastgele dizedir; DB'de
   yalnızca SHA-256 özeti saklanır ve refresh'te **rotasyon** uygulanır (eski iptal,
   yeni verilir). İptal edilebilirlik refresh katmanında, düşük gecikme access
   katmanında (kısa TTL) sağlanır. Argon2id için `golang.org/x/crypto` alındı
   (parola özeti stdlib'de yoktur; bilinçli tek bağımlılık).

3. **Yetki motoru saf çekirdek + DB adaptörü.** `rbac.Decide(effects, roleHas)` saf
   fonksiyondur ve DB'siz tam test edilir; `Evaluator` onu sorgu sonuçlarıyla besler.
   Bu ayrım "otomatik yetki test matrisi"nin CI'da DB olmadan yeşil olmasını sağlar.

4. **Global (proje üstü) izin kontrolü.** `admin.*` gibi proje bağımsız ekranlar için:
   global override'lar (`project_id IS NULL`) + kullanıcının **herhangi** bir
   üyeliğindeki rolün varsayılanı dikkate alınır. Bootstrap admin, tüm izinlerin
   global GRANT'iyle oluşturulur → proje bağımsız superadmin. Proje kapsamlı
   kontrollerde (`projects.manage_members` vb.) o projedeki rol esas alınır.

5. **Audit her yazma işleminde, aynı transaction'da.** `Recorder.RecordTx` iş
   değişikliğiyle audit kaydını atomik bağlar (iş geri alınırsa audit de geri alınır).
   Aktör, auth middleware'i route seviyesinde context'e yazdıktan sonra `MetaFrom`
   tarafından okuma anında birleştirilir (global audit middleware'i yalnızca IP +
   request-id toplar).

6. **`projects` tablosu Faz 1'de oluşturulur.** Proje bazlı yetki ve üyelik için
   gereken çekirdek tanım (§6.2) burada tanımlanır; Faz 2 CRUD/milestone/doküman
   katmanını ekleyerek genişletir. Bu, FK bütünlüğünü Faz 1'den itibaren korur.

## Sonuçlar
- Faz 2+ hazır: `RequirePermission(module.action)` middleware'i, `<Can>` bileşeni,
  audit `RecordTx` sözleşmesi, proje kapsamı çözümü (`ProjectFromRequest`).
- **Şifre sıfırlama e-postası Faz 4 bildirim motoruna bağlıdır.** Faz 1'de jeton
  üretilir, DB'de özeti saklanır ve loglanır (dev'de yanıtta da döner). Uçtan uca
  sözleşme hazır; taşıyıcı (SMTP) Faz 4'te takılır.
- Risk: proje seçici Faz 2'de gelene dek izin matrisi "proje kapsamı" alanı UUID
  girişiyle çalışır; global kapsam varsayılandır.
