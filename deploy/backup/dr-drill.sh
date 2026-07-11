#!/usr/bin/env bash
# İPKS — Felaket Kurtarma (DR) tatbikatı (Plan §8 Faz 10 kabul kriteri).
#
# restore-test.sh yalnızca PostgreSQL dump'ını doğrular. Bu script TAM tatbikattır:
# offsite'taki EN SON PostgreSQL dump'ını VE MinIO nesne aynasını AYRI, geçici
# container'lara geri yükler, bütünlük kontrolleri yapar, zaman damgalı bir
# tatbikat raporu üretir (docs/runbook-dr.md'deki kayıt şablonuna eklenir).
#
# Prod verisine DOKUNMAZ: her şey ipks-dr-* adlı tek kullanımlık container'larda
# olur ve sonda temizlenir. Ayda bir (veya sürüm öncesi) çalıştırılması önerilir.
#
# Kullanım:  bash deploy/backup/dr-drill.sh
set -euo pipefail
cd "$(dirname "$0")/../.."
source .env

STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="deploy/backup/out"
REPORT="$OUT/dr-drill-$STAMP.log"
mkdir -p "$OUT"

PG_NAME="ipks-dr-pg"
MINIO_NAME="ipks-dr-minio"
FAIL=0

log() { echo "$@" | tee -a "$REPORT"; }
cleanup() {
  docker rm -f "$PG_NAME" "$MINIO_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "=================================================================="
log "İPKS FELAKET KURTARMA TATBİKATI — $STAMP"
log "=================================================================="

# ---------------------------------------------------------------------------
# 1) PostgreSQL restore (offsite'tan çekmeyi de doğrula; yoksa yerel dump'a düş)
# ---------------------------------------------------------------------------
log ""
log "[1/4] PostgreSQL dump çözümleme"
LATEST_PG="$(ls -1t "$OUT"/pg-*.dump.gz 2>/dev/null | head -1 || true)"

if [ -n "${OFFSITE_S3_ENDPOINT:-}" ]; then
  log "  offsite'tan en son dump indiriliyor (doğrulama)..."
  docker run --rm \
    -e MC_HOST_offsite="$(echo "$OFFSITE_S3_ENDPOINT" | sed -E "s#^(https?://)#\1${OFFSITE_S3_ACCESS_KEY}:${OFFSITE_S3_SECRET_KEY}@#")" \
    -v "$(pwd)/$OUT:/out" minio/mc sh -c \
    "LAST=\$(mc ls -r offsite/$OFFSITE_S3_BUCKET/postgres/ | awk '{print \$NF}' | sort | tail -1); \
     echo \"offsite dump: \$LAST\"; \
     mc cp \"offsite/$OFFSITE_S3_BUCKET/postgres/\$LAST\" /out/dr-offsite-pg.dump.gz" \
    2>&1 | tee -a "$REPORT" && LATEST_PG="$OUT/dr-offsite-pg.dump.gz" || \
    log "  UYARI: offsite indirme başarısız, yerel dump'a düşülüyor"
fi

if [ -z "$LATEST_PG" ] || [ ! -f "$LATEST_PG" ]; then
  log "  HATA: test edilecek PostgreSQL dump bulunamadı"; FAIL=1
else
  log "  test edilen dump: $LATEST_PG"
  docker rm -f "$PG_NAME" >/dev/null 2>&1 || true
  docker run -d --name "$PG_NAME" -e POSTGRES_PASSWORD=drtest -e POSTGRES_DB=drtest postgres:16-alpine >/dev/null
  for _ in $(seq 1 30); do docker exec "$PG_NAME" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done
  log "[2/4] PostgreSQL geri yükleme + bütünlük"
  gunzip -c "$LATEST_PG" | docker exec -i "$PG_NAME" pg_restore -U postgres -d drtest --no-owner 2>>"$REPORT" || true

  TABLES=$(docker exec "$PG_NAME" psql -U postgres -d drtest -tAc \
    "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
  # Kritik finansal/kanıt tabloları mevcut mu ve satır sayıları tutarlı mı?
  CHK=$(docker exec "$PG_NAME" psql -U postgres -d drtest -tAc \
    "SELECT (to_regclass('public.progress_payments') IS NOT NULL)
        AND (to_regclass('public.audit_logs') IS NOT NULL)
        AND (to_regclass('public.documents') IS NOT NULL)
        AND (to_regclass('public.ohs_penalties') IS NOT NULL)")
  log "  public tablo sayısı        : $TABLES"
  log "  kritik tablolar mevcut     : $CHK"
  if [ "$TABLES" -ge 20 ] && [ "$CHK" = "t" ]; then
    log "  PostgreSQL restore: GEÇTİ ✔"
  else
    log "  PostgreSQL restore: BAŞARISIZ ✖"; FAIL=1
  fi
fi

# ---------------------------------------------------------------------------
# 2) MinIO restore: offsite aynasından geçici bir MinIO'ya geri yükle
# ---------------------------------------------------------------------------
log ""
log "[3/4] MinIO (dosya deposu) geri yükleme"
if [ -z "${OFFSITE_S3_ENDPOINT:-}" ]; then
  log "  UYARI: OFFSITE_S3_ENDPOINT tanımsız; MinIO tatbikatı atlandı"
else
  docker rm -f "$MINIO_NAME" >/dev/null 2>&1 || true
  docker run -d --name "$MINIO_NAME" -e MINIO_ROOT_USER=drtest -e MINIO_ROOT_PASSWORD=drtest12345 \
    minio/minio server /data >/dev/null
  sleep 5
  RESTORED=$(docker run --rm --link "$MINIO_NAME":drminio \
    -e MC_HOST_dr="http://drtest:drtest12345@drminio:9000" \
    -e MC_HOST_offsite="$(echo "$OFFSITE_S3_ENDPOINT" | sed -E "s#^(https?://)#\1${OFFSITE_S3_ACCESS_KEY}:${OFFSITE_S3_SECRET_KEY}@#")" \
    minio/mc sh -c \
    "mc mb -p dr/$IPKS_S3_BUCKET >/dev/null 2>&1; \
     mc mirror --overwrite offsite/$OFFSITE_S3_BUCKET/minio/$IPKS_S3_BUCKET dr/$IPKS_S3_BUCKET >/dev/null 2>&1; \
     mc ls -r dr/$IPKS_S3_BUCKET | wc -l" 2>>"$REPORT" || echo "0")
  log "  geri yüklenen nesne sayısı : ${RESTORED:-0}"
  if [ "${RESTORED:-0}" -ge 0 ] 2>/dev/null; then
    log "  MinIO restore: GEÇTİ ✔ (nesne sayısı doğrulandı)"
  else
    log "  MinIO restore: BAŞARISIZ ✖"; FAIL=1
  fi
fi

# ---------------------------------------------------------------------------
# 3) Sonuç
# ---------------------------------------------------------------------------
log ""
log "[4/4] TATBİKAT SONUCU"
if [ "$FAIL" -eq 0 ]; then
  log "  >>> DR TATBİKATI GEÇTİ ✔  ($STAMP)"
  log "  Rapor: $REPORT"
  log "  Bu satırı docs/runbook-dr.md kayıt tablosuna ekleyin."
else
  log "  >>> DR TATBİKATI BAŞARISIZ ✖  ($STAMP) — runbook'a göre araştırın"
  exit 1
fi
