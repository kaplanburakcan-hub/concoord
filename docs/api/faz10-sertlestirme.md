# Faz 10 — Sertleştirme: API ve Davranış Değişiklikleri

Faz 10 yeni iş uçları eklemez; mevcut API'ye **çapraz kesen güvenlik ve
dayanıklılık davranışları** ekler. İstemci ekiplerinin bilmesi gerekenler:

## 1. Hız sınırlama (429)
İki katman:
- **Genel:** IP başına token-bucket (`IPKS_RATE_LIMIT_RPS`, varsayılan 20/sn,
  burst 40). Aşımda `429 Too Many Requests`.
- **Kimlik uçları:** `/api/v1/auth/{login,refresh,password/forgot,password/reset}`
  için IP başına dakikalık deneme sınırı (`IPKS_LOGIN_RATE_LIMIT`, varsayılan 10).

Yanıt zarfı standart hata formatındadır, yeni kod ile:
```json
{ "error": { "code": "rate_limited",
             "message": "Çok fazla istek gönderildi, lütfen kısa süre sonra tekrar deneyin.",
             "request_id": "..." } }
```
Başlıklar: `Retry-After: 1`, `X-RateLimit-Limit: <n>`.
**İstemci önerisi:** 429'da üstel geri çekilme (backoff) ile yeniden dene.

## 2. CORS
Aynı-origin dağıtımda değişiklik yok. Ayrı origin gerekiyorsa
`IPKS_CORS_ORIGINS` allowlist'ine eklenir; joker (`*`) desteklenmez. Preflight
(`OPTIONS`) `204` döner. İzinli origin'e `Access-Control-Allow-Credentials: true`
verilir (Authorization başlığı taşınabilir).

## 3. Güvenlik başlıkları
Tüm yanıtlarda: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: strict-origin-when-cross-origin`,
`Cross-Origin-Opener-Policy: same-origin`.

## 4. Yükleme doğrulaması (415)
Doküman yükleme (`POST .../documents/{id}/versions`) ve İSG fotoğraf yükleme
(data-URL) artık **içerik koklaması (magic byte)** ile doğrulanır:
- Tarayıcının bildirdiği `Content-Type`'a güvenilmez; ilk 512 bayttan çıkarılan
  gerçek tip saklanır.
- İzinli uzantı allowlist'i: pdf, png/jpg/jpeg/gif/webp/heic/heif, csv/txt,
  xlsx/docx/xls/doc, dwg.
- İçerik ↔ uzantı uyuşmazlığında veya yürütülebilir içerikte:
  `415 Unsupported Media Type`, `code: validation_error`.
- `IPKS_CLAMD_ADDR` ayarlıysa ek olarak ClamAV taraması; virüs bulunursa yükleme
  reddedilir (fail-closed: clamd erişilemezse de reddedilir).

## 5. İzleme uçları (değişmedi, teyit)
- `GET /healthz` → süreç canlı (`200`).
- `GET /readyz` → DB erişilebilir (`200`) / değilse `503`.
- 5xx/panik olayları opsiyonel webhook'a (`IPKS_ERROR_WEBHOOK_URL`) bildirilir.

## 6. Sürüm
`GET /api/v1/meta` → `{"version": "1.0.0", ...}`.
