COMPOSE=docker compose -f deploy/docker-compose.yml --env-file .env

.PHONY: up down logs build migrate-up migrate-down migrate-new seed demo-data test test-race fmt-check lint-frontend test-frontend migrate-smoke api-smoke ci backup restore-test dr-drill loadtest smoke psql

up: ## Geliştirme ortamını ayağa kaldır
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=100

build:
	$(COMPOSE) build

migrate-up: ## Tüm migration'ları uygula
	$(COMPOSE) --profile tools run --rm migrate up

migrate-down: ## Son migration'ı geri al
	$(COMPOSE) --profile tools run --rm migrate down 1

seed: ## Seed verisini yükle (idempotent): izin sözlüğü + 7 rol + bootstrap admin
	$(COMPOSE) exec api /app/api -seed

demo-data: ## Gerçekçi DEMO-01 test verisi yükle (EVM/S-eğrisi, İSG, MAR, görevler)
	$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-ipks} -d $${POSTGRES_DB:-ipks} < deploy/seed/demo-data.sql

test: ## Backend birim testleri (auth, JWT, RBAC yetki matrisi)
	cd backend && go test ./...

# --- Kalite kapısı (CI ile aynı kontroller; bkz. docs/kalite-kapisi.md) ---

test-race: ## Backend testleri race detector ile
	cd backend && go test -race -count=1 ./...

fmt-check: ## gofmt + go vet
	@cd backend && test -z "$$(gofmt -l .)" || (echo "gofmt gerekli: cd backend && gofmt -w ." && gofmt -l . && exit 1)
	cd backend && go vet ./...

lint-frontend: ## Frontend type-check + ESLint
	cd frontend && npm run typecheck && npm run lint

test-frontend: ## Frontend birim/bileşen testleri (Vitest)
	cd frontend && npm test

migrate-smoke: ## Temiz DB'de migration up→down→up (IPKS_DB_DSN boş bir DB'ye işaret etmeli)
	bash deploy/ci/migration-smoke.sh

api-smoke: ## Çalışan API'ye karşı smoke/kontrat testleri (BASE_URL, ADMIN_EMAIL, ADMIN_PASS)
	bash deploy/ci/api-smoke.sh

ci: fmt-check test-race lint-frontend test-frontend ## DB gerektirmeyen CI kontrollerini yerelde çalıştır
	cd frontend && npm run build

backup: ## Manuel yedek al (offsite dahil)
	bash deploy/backup/backup.sh

restore-test: ## Son yedeği ayrı container'da geri yükleyip doğrula
	bash deploy/backup/restore-test.sh

dr-drill: ## Faz 10 — tam felaket kurtarma tatbikatı (PostgreSQL + MinIO restore)
	bash deploy/backup/dr-drill.sh

smoke: ## Faz 10 — k6 duman testi (BASE_URL, IPKS_USER, IPKS_PASS ortam değişkenleriyle)
	k6 run deploy/loadtest/k6-smoke.js

loadtest: ## Faz 10 — k6 yük testi, 100 eşzamanlı kullanıcı
	k6 run deploy/loadtest/k6-load.js

psql:
	$(COMPOSE) exec postgres psql -U ipks ipks
