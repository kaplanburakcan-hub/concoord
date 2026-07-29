#!/bin/sh
set -e
echo "Migration'lar uygulanıyor..."
migrate -path=/app/migrations -database "${IPKS_DB_DSN}" up
echo "Seed adımları uygulanıyor..."
/app/api -seed
echo "API başlatılıyor..."
exec /app/api
