#!/usr/bin/env bash
# Migration smoke testi — temiz PostgreSQL üzerinde up → down → up round-trip.
#
# Kullanım:
#   IPKS_DB_DSN='postgres://ipks:ipks@localhost:5432/ipks?sslmode=disable' \
#     bash deploy/ci/migration-smoke.sh
#
# Ortam değişkenleri:
#   IPKS_DB_DSN      (zorunlu) BOŞ bir veritabanına işaret etmeli
#   MIGRATE_BIN      migrate CLI yolu (varsayılan: PATH'teki `migrate`)
#   MIGRATIONS_DIR   (varsayılan: backend/migrations)
#
# Kontroller:
#   1. Her NNNNNN_x.up.sql için eşleşen .down.sql var (ve tersi).
#   2. Sürüm numaraları benzersiz.
#   3. `up` tüm migration'ları hatasız uygular, son sürüm = en büyük dosya no.
#   4. `down -all` şemayı tamamen geri alır (yalnızca schema_migrations kalır).
#   5. İkinci `up` yeniden hatasız uygular (down'lar gerçekten tersine çeviriyor).
set -euo pipefail

: "${IPKS_DB_DSN:?IPKS_DB_DSN zorunludur}"
MIGRATE_BIN="${MIGRATE_BIN:-migrate}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT/backend/migrations}"

fail() { echo "HATA: $*" >&2; exit 1; }
step() { echo; echo "==> $*"; }
mig()  { "$MIGRATE_BIN" -path "$MIGRATIONS_DIR" -database "$IPKS_DB_DSN" "$@"; }
sql()  { psql "$IPKS_DB_DSN" -v ON_ERROR_STOP=1 -Atc "$1"; }

step "Dosya çiftleri kontrol ediliyor ($MIGRATIONS_DIR)"
shopt -s nullglob
missing=0
for up in "$MIGRATIONS_DIR"/*.up.sql; do
  down="${up%.up.sql}.down.sql"
  [[ -f "$down" ]] || { echo "  eksik down: $(basename "$down")"; missing=1; }
done
for down in "$MIGRATIONS_DIR"/*.down.sql; do
  up="${down%.down.sql}.up.sql"
  [[ -f "$up" ]] || { echo "  eksik up: $(basename "$up")"; missing=1; }
done
[[ $missing -eq 0 ]] || fail "up/down çiftleri eksik"

dups=$(for f in "$MIGRATIONS_DIR"/*.up.sql; do basename "$f" | cut -d_ -f1; done | sort | uniq -d)
[[ -z "$dups" ]] || fail "tekrarlı migration sürümü: $dups"

latest=$(for f in "$MIGRATIONS_DIR"/*.up.sql; do basename "$f" | cut -d_ -f1; done | sort -n | tail -1)
latest=$((10#$latest))
echo "  $(ls "$MIGRATIONS_DIR"/*.up.sql | wc -l) migration, son sürüm: $latest"

step "Veritabanı boş mu?"
tables=$(sql "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations'")
[[ "$tables" == "0" ]] || fail "veritabanı boş değil ($tables tablo) — temiz bir DB gerekli"

check_version() {
  local got
  got=$(sql "SELECT version || ':' || CASE WHEN dirty THEN 'dirty' ELSE 'clean' END FROM schema_migrations")
  [[ "$got" == "$latest:clean" ]] || fail "beklenen sürüm $latest (temiz), bulunan: $got"
}

step "migrate up (1. tur)"
mig up
check_version

step "migrate down -all"
mig down -all
left=$(sql "SELECT string_agg(tablename, ', ') FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations'")
[[ -z "$left" ]] || fail "down -all sonrası kalan tablolar: $left"
types=$(sql "SELECT string_agg(t.typname, ', ') FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typtype IN ('e','d')")
[[ -z "$types" ]] || fail "down -all sonrası kalan enum/domain tipleri: $types"

step "migrate up (2. tur)"
mig up
check_version

echo
echo "OK — migration round-trip başarılı (son sürüm $latest)."
