#!/usr/bin/env bash
# İPKS — günlük yedek: pg_dump + WAL arşivi + MinIO → offsite S3
# İlke (Plan §5.3): VPS tek hata noktasıdır; offsite hedef ZORUNLUDUR.
# Saklama: 30 gün günlük + 12 ay aylık (ayın 1'i "aylık" sayılır).
set -euo pipefail
cd "$(dirname "$0")/../.."
source .env

STAMP=$(date +%Y%m%d-%H%M%S)
DAY=$(date +%d)
OUT="deploy/backup/out"
mkdir -p "$OUT"
COMPOSE="docker compose -f deploy/docker-compose.yml --env-file .env"
FAIL=0

notify_fail() {
  # Başarısız yedek admin'e bildirim üretir.
  # Faz 4'te bildirim motoruna bağlanır; şimdilik log + stderr.
  echo "[YEDEK HATASI] $1" >&2
  logger -t ipks-backup "HATA: $1" || true
}

echo "[1/4] PostgreSQL dump"
if $COMPOSE exec -T postgres pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB" > "$OUT/pg-$STAMP.dump"; then
  gzip -f "$OUT/pg-$STAMP.dump"
else
  notify_fail "pg_dump başarısız"; FAIL=1
fi

echo "[2/4] Offsite alias + dump ve WAL arşivi gönderimi"
MC="docker run --rm --network ipks_default \
    -v $(pwd)/$OUT:/out -v ipks_wal_archive:/wal_archive:ro \
    -e MC_HOST_offsite=$(echo "$OFFSITE_S3_ENDPOINT" | sed -E "s#^(https?://)#\1${OFFSITE_S3_ACCESS_KEY}:${OFFSITE_S3_SECRET_KEY}@#") \
    minio/mc"

if [ "${DAY}" = "01" ]; then TIER="monthly"; else TIER="daily"; fi
$MC cp "/out/pg-$STAMP.dump.gz" "offsite/$OFFSITE_S3_BUCKET/postgres/$TIER/pg-$STAMP.dump.gz" \
  || { notify_fail "dump offsite'a gönderilemedi"; FAIL=1; }
$MC mirror --overwrite /wal_archive "offsite/$OFFSITE_S3_BUCKET/wal_archive" \
  || { notify_fail "WAL arşivi gönderilemedi"; FAIL=1; }

echo "[3/4] MinIO (dosya deposu) → offsite mirror"
docker run --rm --network ipks_default \
  -e MC_HOST_src="http://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@minio:9000" \
  -e MC_HOST_offsite=$(echo "$OFFSITE_S3_ENDPOINT" | sed -E "s#^(https?://)#\1${OFFSITE_S3_ACCESS_KEY}:${OFFSITE_S3_SECRET_KEY}@#") \
  minio/mc mirror --overwrite "src/$IPKS_S3_BUCKET" "offsite/$OFFSITE_S3_BUCKET/minio/$IPKS_S3_BUCKET" \
  || { notify_fail "MinIO mirror başarısız"; FAIL=1; }

echo "[4/4] Yerel saklama temizliği (30 gün)"
find "$OUT" -name 'pg-*.dump.gz' -mtime +30 -delete || true

if [ "$FAIL" -eq 0 ]; then
  echo "Yedek tamam: pg-$STAMP.dump.gz (tier: $TIER)"
else
  exit 1
fi
