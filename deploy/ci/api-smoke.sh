#!/usr/bin/env bash
# API smoke + kontrat testi — çalışan bir API'ye karşı kritik uçları doğrular.
#
# Kullanım:
#   BASE_URL=http://localhost:8080 ADMIN_EMAIL=admin@ipks.local ADMIN_PASS=... \
#     bash deploy/ci/api-smoke.sh
#
# Bağımlılıklar: curl, jq
#
# Doğrulananlar:
#   - /healthz, /readyz, /api/v1/meta yanıt şekilleri
#   - Hata zarfı kontratı: {"error":{"code","message","request_id"}}
#   - Kimlik doğrulama: yanlış parola → 401, doğru parola → token çifti
#   - Kimliksiz erişim → 401, yetkisiz erişim (RBAC) → 403
#   - Admin için kritik liste uçları → 200 + beklenen JSON tipi
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@ipks.local}"
: "${ADMIN_PASS:?ADMIN_PASS zorunludur}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
failures=0
body="$TMP/body.json"

# req METHOD PATH [TOKEN] [JSON] → HTTP durum kodunu yazar, gövde $body'de.
req() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}"
  local args=(-sS -o "$body" -w '%{http_code}' -X "$method" "$BASE_URL$path"
              -H 'Accept: application/json')
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$data" ]] && args+=(-H 'Content-Type: application/json' --data "$data")
  curl "${args[@]}"
}

# expect AÇIKLAMA BEKLENEN_KOD GERÇEK_KOD [JQ_İFADESİ]
expect() {
  local name="$1" want="$2" got="$3" filter="${4:-}"
  if [[ "$got" != "$want" ]]; then
    echo "  FAIL  $name — HTTP $got (beklenen $want): $(head -c 300 "$body")"
    failures=$((failures + 1)); return
  fi
  if [[ -n "$filter" ]] && ! jq -e "$filter" "$body" >/dev/null 2>&1; then
    echo "  FAIL  $name — kontrat ihlali ($filter): $(head -c 300 "$body")"
    failures=$((failures + 1)); return
  fi
  echo "  ok    $name"
}

ERR_ENVELOPE='(.error.code|type=="string") and (.error.message|type=="string") and (.error.request_id|type=="string")'

echo "==> Sağlık uçları ($BASE_URL)"
expect "GET /healthz"      200 "$(req GET /healthz)"      '.status=="ok"'
expect "GET /readyz"       200 "$(req GET /readyz)"       '.status=="ready"'
expect "GET /api/v1/meta"  200 "$(req GET /api/v1/meta)"  '(.name|type=="string") and (.version|type=="string")'

echo "==> Kimlik doğrulama"
expect "GET /auth/me kimliksiz → 401" 401 "$(req GET /api/v1/auth/me)" "$ERR_ENVELOPE"
bad=$(jq -nc --arg i "$ADMIN_EMAIL" '{identifier:$i, password:"yanlis-parola-xyz"}')
expect "POST /auth/login yanlış parola → 401" 401 "$(req POST /api/v1/auth/login '' "$bad")" "$ERR_ENVELOPE"
good=$(jq -nc --arg i "$ADMIN_EMAIL" --arg p "$ADMIN_PASS" '{identifier:$i, password:$p}')
expect "POST /auth/login admin → 200" 200 "$(req POST /api/v1/auth/login '' "$good")" \
  '(.tokens.access_token|type=="string") and (.tokens.refresh_token|type=="string") and (.user.id|type=="string")'
ADMIN_TOKEN=$(jq -r '.tokens.access_token // empty' "$body")
[[ -n "$ADMIN_TOKEN" ]] || { echo "Admin token alınamadı, devam edilemiyor."; exit 1; }
expect "GET /auth/me admin → 200" 200 "$(req GET /api/v1/auth/me "$ADMIN_TOKEN")" '.'

echo "==> Kritik liste uçları (admin)"
expect "GET /projects"            200 "$(req GET /api/v1/projects "$ADMIN_TOKEN")"            '.'
expect "GET /admin/users"         200 "$(req GET /api/v1/admin/users "$ADMIN_TOKEN")"         '.'
expect "GET /admin/roles"         200 "$(req GET /api/v1/admin/roles "$ADMIN_TOKEN")"         '.'
expect "GET /admin/permissions"   200 "$(req GET /api/v1/admin/permissions "$ADMIN_TOKEN")"   '.'
expect "GET /admin/audit-logs"    200 "$(req GET /api/v1/admin/audit-logs "$ADMIN_TOKEN")"    '.'

echo "==> RBAC: rolsüz kullanıcı admin uçlarına erişememeli"
suffix="$(date +%s)$RANDOM"
user_email="ci-smoke-$suffix@ipks.local"
user_pass="CiSmoke!${suffix}Aa"
new_user=$(jq -nc --arg e "$user_email" --arg u "ci_smoke_$suffix" --arg p "$user_pass" \
  '{email:$e, username:$u, full_name:"CI Smoke", password:$p, is_active:true}')
code=$(req POST /api/v1/admin/users "$ADMIN_TOKEN" "$new_user")
if [[ "$code" == "200" || "$code" == "201" ]]; then
  echo "  ok    POST /admin/users (rolsüz test kullanıcısı)"
  login=$(jq -nc --arg i "$user_email" --arg p "$user_pass" '{identifier:$i, password:$p}')
  expect "POST /auth/login test kullanıcısı → 200" 200 "$(req POST /api/v1/auth/login '' "$login")" '.tokens.access_token|type=="string"'
  USER_TOKEN=$(jq -r '.tokens.access_token // empty' "$body")
  if [[ -n "$USER_TOKEN" ]]; then
    expect "GET /admin/users rolsüz → 403"      403 "$(req GET /api/v1/admin/users "$USER_TOKEN")"      "$ERR_ENVELOPE"
    expect "GET /admin/audit-logs rolsüz → 403" 403 "$(req GET /api/v1/admin/audit-logs "$USER_TOKEN")" "$ERR_ENVELOPE"
    expect "POST /projects rolsüz → 403"        403 "$(req POST /api/v1/projects "$USER_TOKEN" '{"code":"X","name":"X"}')" "$ERR_ENVELOPE"
  fi
else
  echo "  FAIL  POST /admin/users — HTTP $code: $(head -c 300 "$body")"
  failures=$((failures + 1))
fi

echo
if [[ $failures -gt 0 ]]; then
  echo "API smoke: $failures kontrol başarısız."
  exit 1
fi
echo "OK — API smoke testleri başarılı."
