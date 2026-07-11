#!/usr/bin/env bash
# İPKS — prod dağıtım: build → migrate → up → sağlık kontrolü
# Kullanım: repo kökünden `bash deploy/scripts/deploy.sh`
set -euo pipefail
cd "$(dirname "$0")/../.."

COMPOSE="docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml --env-file .env"

echo "[1/4] İmajlar derleniyor"
$COMPOSE build

echo "[2/4] Migration'lar uygulanıyor"
$COMPOSE up -d postgres
$COMPOSE --profile tools run --rm migrate up

echo "[3/4] Servisler başlatılıyor"
$COMPOSE up -d

echo "[4/4] Sağlık kontrolü"
sleep 5
source .env
if curl -fsSk "https://${DOMAIN}/readyz" >/dev/null 2>&1 || curl -fsS "http://127.0.0.1:8080/readyz" >/dev/null 2>&1; then
  echo "Dağıtım başarılı ✔"
else
  echo "UYARI: readyz yanıt vermedi — 'make logs' ile inceleyin." >&2
  exit 1
fi
