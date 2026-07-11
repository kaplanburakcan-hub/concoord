#!/usr/bin/env bash
# İPKS — VPS ilk kurulum (Ubuntu 22.04/24.04)
# Kullanım: sudo bash deploy/scripts/vps-setup.sh
set -euo pipefail

echo "[1/5] Sistem güncelleme + temel paketler"
apt-get update -y
apt-get install -y ca-certificates curl git ufw

echo "[2/5] Docker kurulumu"
if ! command -v docker >/dev/null; then
  curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker

echo "[3/5] Güvenlik duvarı (yalnızca SSH + HTTP/HTTPS)"
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "[4/5] Uygulama dizini"
mkdir -p /opt/ipks
echo "  → Repoyu /opt/ipks içine klonlayın ve .env.example'dan .env oluşturun."

echo "[5/5] Yedek cron'u"
echo "  → deploy/backup/crontab.example içeriğini 'crontab -e' ile ekleyin."

echo "Kurulum tamam. Sonraki adımlar:"
echo "  1) cd /opt/ipks && cp .env.example .env && düzenleyin (DOMAIN, parolalar, offsite S3)"
echo "  2) bash deploy/scripts/init-letsencrypt.sh   # ilk sertifika"
echo "  3) bash deploy/scripts/deploy.sh             # sistemi ayağa kaldır"
