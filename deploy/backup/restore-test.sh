#!/usr/bin/env bash
# İPKS — otomatik restore testi (Plan §5.3: "Test edilmeyen yedek, yedek değildir")
# Son dump'ı AYRI bir geçici PostgreSQL container'ına geri yükler,
# temel bütünlük sorguları çalıştırır, sonucu raporlar/loglar.
set -euo pipefail
cd "$(dirname "$0")/../.."
source .env

OUT="deploy/backup/out"
REPORT="deploy/backup/out/restore-report-$(date +%Y%m%d-%H%M%S).log"
LATEST=$(ls -1t "$OUT"/pg-*.dump.gz 2>/dev/null | head -1 || true)

if [ -z "$LATEST" ]; then
  echo "HATA: test edilecek dump bulunamadı ($OUT). Önce 'make backup' çalıştırın." | tee "$REPORT" >&2
  exit 1
fi
echo "Test edilen yedek: $LATEST" | tee "$REPORT"

NAME="ipks-restore-test"
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" -e POSTGRES_PASSWORD=test -e POSTGRES_DB=restoretest postgres:16-alpine >/dev/null

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "Container hazırlanıyor..." | tee -a "$REPORT"
for i in $(seq 1 30); do
  docker exec "$NAME" pg_isready -U postgres >/dev/null 2>&1 && break
  sleep 2
done

echo "Geri yükleme..." | tee -a "$REPORT"
gunzip -c "$LATEST" | docker exec -i "$NAME" pg_restore -U postgres -d restoretest --no-owner

echo "Bütünlük kontrolleri:" | tee -a "$REPORT"
TABLES=$(docker exec "$NAME" psql -U postgres -d restoretest -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
AUDIT_OK=$(docker exec "$NAME" psql -U postgres -d restoretest -tAc \
  "SELECT to_regclass('public.audit_logs') IS NOT NULL")

echo "  public tablo sayısı : $TABLES" | tee -a "$REPORT"
echo "  audit_logs mevcut   : $AUDIT_OK" | tee -a "$REPORT"

if [ "$TABLES" -ge 2 ] && [ "$AUDIT_OK" = "t" ]; then
  echo "SONUÇ: RESTORE TESTİ GEÇTİ ✔" | tee -a "$REPORT"
else
  echo "SONUÇ: RESTORE TESTİ BAŞARISIZ ✖" | tee -a "$REPORT" >&2
  exit 1
fi
