#!/bin/sh
set -e
echo "Migration'lar uygulanıyor..."
# Dirty state varsa bir önceki sürüme zorla, ardından tekrar dene
if ! migrate -path=/app/migrations -database "${IPKS_DB_DSN}" up 2>/tmp/migrate_err; then
    if grep -q "Dirty database version" /tmp/migrate_err; then
        DIRTY_VER=$(grep -o 'version [0-9]*' /tmp/migrate_err | grep -o '[0-9]*')
        PREV_VER=$((DIRTY_VER - 1))
        echo "Dirty version ${DIRTY_VER} tespit edildi, ${PREV_VER}'e geri alınıyor..."
        migrate -path=/app/migrations -database "${IPKS_DB_DSN}" force ${PREV_VER}
        migrate -path=/app/migrations -database "${IPKS_DB_DSN}" up
    else
        cat /tmp/migrate_err
        exit 1
    fi
fi
echo "Seed adımları uygulanıyor..."
/app/api -seed
echo "API başlatılıyor..."
exec /app/api
