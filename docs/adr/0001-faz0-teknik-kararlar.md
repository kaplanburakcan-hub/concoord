# ADR-0001 — Faz 0 Teknik Kararları

**Tarih:** 06.07.2026 · **Durum:** Kabul edildi

## Kararlar

1. **Audit kaydı repository katmanında, HTTP katmanında değil.** HTTP middleware yalnızca meta (aktör, IP, request-id) toplar; `before/after` diff'i ancak veri katmanında bilinir. Böylece "geliştirici unutamaz" ilkesi Faz 1'de repository sözleşmesiyle zorlanır.
2. **Seed = Go kodu + `seed_history` tablosu (SQL dosyası değil).** Rol/izin seed'leri (Faz 1) koşullu mantık gerektirir; idempotentlik tabloyla garanti edilir, migration'lardan ayrı yaşar.
3. **WAL arşivleme `archive_command` ile yerel volume'a, offsite'a `mc mirror` ile.** Ek araç (wal-g) bağımlılığı Faz 0'da alınmadı; dump + WAL kombinasyonu plan §5.3 hedefini karşılar. wal-g'ye geçiş backlog notu.
4. **Prod'da hiçbir servis host'a port açmaz; tek giriş nginx (80/443).** Dev'de portlar 127.0.0.1'e bağlıdır.
5. **Worker Faz 0'da iskelet (ping döngüsü).** river kuyruğu ilk gerçek işle (Faz 4 bildirim/PDF) birlikte eklenir — kullanılmayan bağımlılık taşınmaz.

## Sonuçlar
- Faz 1 başlangıcında hazır: config, log, hata zarfı, audit Recorder, seed Registry, migration hattı.
- Riskler: offsite S3 kimlikleri girilmeden `backup.sh` başarısız olur ve bunu raporlar (bilinçli — sessiz başarısızlık yok).
