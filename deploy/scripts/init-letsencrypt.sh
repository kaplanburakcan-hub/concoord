#!/usr/bin/env bash
# İlk Let's Encrypt sertifikasını alır (nginx henüz sertifikasızken tavuk-yumurta çözümü).
# Kullanım: repo kökünden `bash deploy/scripts/init-letsencrypt.sh`
set -euo pipefail
cd "$(dirname "$0")/../.."
source .env

COMPOSE="docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml --env-file .env"
mkdir -p deploy/certbot/www deploy/certbot/conf

echo "[1/3] Geçici self-signed sertifika (nginx'in ayağa kalkabilmesi için)"
CERTDIR="deploy/certbot/conf/live/${DOMAIN}"
if [ ! -f "${CERTDIR}/fullchain.pem" ]; then
  mkdir -p "${CERTDIR}"
  docker run --rm -v "$(pwd)/deploy/certbot/conf:/etc/letsencrypt" alpine/openssl req -x509 -nodes \
    -newkey rsa:2048 -days 1 \
    -keyout "/etc/letsencrypt/live/${DOMAIN}/privkey.pem" \
    -out "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" \
    -subj "/CN=${DOMAIN}"
fi

echo "[2/3] nginx başlatılıyor"
$COMPOSE up -d nginx

echo "[3/3] Gerçek sertifika alınıyor"
docker run --rm \
  -v "$(pwd)/deploy/certbot/conf:/etc/letsencrypt" \
  -v "$(pwd)/deploy/certbot/www:/var/www/certbot" \
  certbot/certbot certonly --webroot -w /var/www/certbot \
  --email "${LETSENCRYPT_EMAIL}" -d "${DOMAIN}" \
  --agree-tos --no-eff-email --force-renewal

$COMPOSE exec nginx nginx -s reload
echo "Sertifika hazır: https://${DOMAIN}"
