# İPKS Yük Testi (Faz 10)

Kabul kriteri (Plan §8, Faz 10): **100 eşzamanlı kullanıcı** altında sistemin
kabul edilebilir gecikmeyle ve hatasız çalıştığının gösterilmesi.

## Gereksinim
- [k6](https://k6.io) (`brew install k6` / `apt install k6` / Docker imajı `grafana/k6`)
- Erişilebilir bir İPKS ortamı (staging önerilir; prod'da düşük trafikli pencere)
- Test kullanıcısı kimlik bilgileri

## Çalıştırma

Önce duman testi (uçlar ayakta mı):

```bash
BASE_URL=https://staging.ipks.example.com \
IPKS_USER=admin@ipks.local IPKS_PASS='***' \
k6 run deploy/loadtest/k6-smoke.js
```

Sonra tam yük testi (100 VU):

```bash
BASE_URL=https://staging.ipks.example.com \
IPKS_USER=admin@ipks.local IPKS_PASS='***' \
k6 run deploy/loadtest/k6-load.js
```

Docker ile (k6 kurulu değilse):

```bash
docker run --rm -i \
  -e BASE_URL=https://staging.ipks.example.com \
  -e IPKS_USER=admin@ipks.local -e IPKS_PASS='***' \
  -v "$PWD/deploy/loadtest:/scripts" \
  grafana/k6 run /scripts/k6-load.js
```

## Eşikler (thresholds)
`k6-load.js` içinde tanımlı; aşılırsa k6 sıfırdan farklı kod döner (CI'da kırmızı):

| Metrik | Eşik |
|---|---|
| `http_req_failed` | < %1 |
| `http_req_duration p95` | < 800 ms |
| `http_req_duration p99` | < 2000 ms |

## Rate limit etkileşimi
Genel hız sınırı (`IPKS_RATE_LIMIT_RPS`, varsayılan 20/sn per-IP) yük testinde
tetiklenebilir; tüm sanal kullanıcılar aynı çıkış IP'sinden gelirse 429 alırsınız.
Yük testi sırasında ya sınırı geçici yükseltin ya da testi birden çok kaynaktan
dağıtın. Kimlik uçları sınırı (`IPKS_LOGIN_RATE_LIMIT`, varsayılan 10/dk) nedeniyle
senaryo `setup()` içinde **tek kez** login olur ve token'ı paylaşır.

## Profil
Okuma-ağırlıklı (login → projeler → portföy → dashboard/EVM → görevler).
Dashboard/EVM toplama sorguları en pahalı yoldur; Faz 10 performans indeksleri
(`000011_faz10_performance`) bu yolu hedefler.
