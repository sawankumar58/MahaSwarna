# ─── MahaSwarna Backend — Makefile ─────────────────────────────────────────
# All targets assume you are in mahaswarna_backend/.
# Prerequisites: go 1.22+, docker, docker-compose, migrate (golang-migrate CLI),
#                golangci-lint, gotest (optional for pretty output).
#
# Quick start:
#   make dev          — start infra (Postgres, Redis, MinIO) + run all services
#   make test         — run all unit tests
#   make lint         — run golangci-lint
#   make migrate-up   — apply all pending migrations (all services)
#   make migrate-down — roll back last migration (all services)

SHELL := /bin/bash
.DEFAULT_GOAL := help

# ── Variables ─────────────────────────────────────────────────────────────────

GO        := go
GOTEST    := $(shell which gotest 2>/dev/null || echo go test)
LINT      := golangci-lint
MIGRATE   := migrate
DOCKER    := docker
COMPOSE   := docker compose

# Services and their DB migration paths
SERVICES := core pricing intelligence gateway
MIGRATE_DIR := ./migrations

# Postgres DSN (overridable via env — reads .env.example defaults)
DB_USER     ?= mahaswarna
DB_PASS     ?= changeme
DB_HOST     ?= localhost
DB_PORT     ?= 5432
DB_NAME     ?= mahaswarna
DATABASE_URL ?= postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# ── Help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── Infrastructure ────────────────────────────────────────────────────────────

.PHONY: infra-up
infra-up: ## Start Postgres, Redis Sentinel, MinIO via docker-compose
	$(COMPOSE) up -d postgres redis-master redis-replica \
	  redis-sentinel-1 redis-sentinel-2 redis-sentinel-3 minio

.PHONY: infra-down
infra-down: ## Stop all infra containers (data preserved)
	$(COMPOSE) stop postgres redis-master redis-replica \
	  redis-sentinel-1 redis-sentinel-2 redis-sentinel-3 minio

.PHONY: infra-reset
infra-reset: ## DESTRUCTIVE: remove infra containers and volumes (wipes all local data)
	@read -p "This will DELETE all local Postgres and Redis data. Type 'yes' to confirm: " c; \
	  [ "$$c" = "yes" ] && $(COMPOSE) down -v || echo "Aborted."

# ── Development ───────────────────────────────────────────────────────────────

.PHONY: dev
dev: infra-up migrate-up ## Start infra + apply migrations + run all services (air hot-reload)
	@echo "Starting services with hot-reload..."
	@for svc in $(SERVICES); do \
	  if [ "$$svc" = "gateway" ]; then \
	    (cd src/gateway && air -c .air.toml &); \
	  else \
	    (cd src/services/$$svc && air -c .air.toml &); \
	  fi; \
	done; \
	wait

.PHONY: run-gateway
run-gateway: ## Run only the gateway service (no hot-reload)
	$(GO) run ./src/gateway/...

.PHONY: run-core
run-core: ## Run only the core service
	$(GO) run ./src/services/core/...

.PHONY: run-pricing
run-pricing: ## Run only the pricing service
	$(GO) run ./src/services/pricing/...

.PHONY: run-intelligence
run-intelligence: ## Run only the intelligence service
	$(GO) run ./src/services/intelligence/...

# ── Testing ───────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all unit tests with race detector
	$(GOTEST) -race -count=1 ./...

.PHONY: test-core
test-core: ## Run core service tests only
	$(GOTEST) -race -count=1 ./test/core/... ./src/services/core/...

.PHONY: test-pricing
test-pricing: ## Run pricing service tests only
	$(GOTEST) -race -count=1 ./test/pricing/... ./src/services/pricing/...

.PHONY: test-gateway
test-gateway: ## Run gateway tests only
	$(GOTEST) -race -count=1 ./test/gateway/... ./src/gateway/...

.PHONY: test-intelligence
test-intelligence: ## Run intelligence service tests only
	$(GOTEST) -race -count=1 ./test/intelligence/... ./src/services/intelligence/...

.PHONY: test-cover
test-cover: ## Run tests with coverage report (opens browser)
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: test-integration
test-integration: ## Run integration tests (requires running infra)
	$(GOTEST) -race -count=1 -tags=integration ./...

# ── Linting & Formatting ─────────────────────────────────────────────────────

.PHONY: lint
lint: ## Run golangci-lint
	$(LINT) run ./...

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with --fix (auto-fix where possible)
	$(LINT) run --fix ./...

.PHONY: fmt
fmt: ## Run gofmt and goimports
	gofmt -w ./src ./test
	@which goimports > /dev/null && goimports -w ./src ./test || true

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: check
check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

# ── Database Migrations ───────────────────────────────────────────────────────

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations for all services
	@echo "Applying migrations..."
	@for svc in core pricing intelligence; do \
	  echo "  → $$svc"; \
	  $(MIGRATE) -path $(MIGRATE_DIR)/$$svc -database "$(DATABASE_URL)" up; \
	done

.PHONY: migrate-down
migrate-down: ## Roll back the last migration for all services
	@echo "Rolling back last migration..."
	@for svc in core pricing intelligence; do \
	  echo "  → $$svc"; \
	  $(MIGRATE) -path $(MIGRATE_DIR)/$$svc -database "$(DATABASE_URL)" down 1; \
	done

.PHONY: migrate-status
migrate-status: ## Show migration status for all services
	@for svc in core pricing intelligence; do \
	  echo "=== $$svc ==="; \
	  $(MIGRATE) -path $(MIGRATE_DIR)/$$svc -database "$(DATABASE_URL)" version 2>&1 || true; \
	done

.PHONY: migrate-create
migrate-create: ## Create new migration: make migrate-create SERVICE=core NAME=add_foo_table
	@[ -n "$(SERVICE)" ] || (echo "Usage: make migrate-create SERVICE=core NAME=add_foo_table"; exit 1)
	@[ -n "$(NAME)" ]    || (echo "Usage: make migrate-create SERVICE=core NAME=add_foo_table"; exit 1)
	$(MIGRATE) create -ext sql -dir $(MIGRATE_DIR)/$(SERVICE) -seq $(NAME)
	@echo "Created migration in $(MIGRATE_DIR)/$(SERVICE)"

# ── JWT Key Rotation ──────────────────────────────────────────────────────────

.PHONY: jwt-rotate
jwt-rotate: ## Zero-downtime JWT RS256 key rotation (see scripts/jwt_rotate.sh)
	./scripts/jwt_rotate.sh

# ── Database Backup ───────────────────────────────────────────────────────────

.PHONY: db-backup
db-backup: ## Create a PostgreSQL dump to ./backups/ (see scripts/db_backup.sh)
	./scripts/db_backup.sh

# ── Docker Builds ─────────────────────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build all service Docker images
	$(DOCKER) build -f Dockerfile.gateway     -t mahaswarna-gateway     .
	$(DOCKER) build -f Dockerfile.core        -t mahaswarna-core        .
	$(DOCKER) build -f Dockerfile.pricing     -t mahaswarna-pricing     .
	$(DOCKER) build -f Dockerfile.intelligence -t mahaswarna-intelligence .

.PHONY: docker-build-gateway
docker-build-gateway: ## Build gateway image only
	$(DOCKER) build -f Dockerfile.gateway -t mahaswarna-gateway .

.PHONY: docker-up
docker-up: ## Start full stack via docker-compose.yml (dev with infra)
	$(COMPOSE) up --build

.PHONY: docker-prod
docker-prod: ## Start production stack via docker-compose.prod.yml
	$(COMPOSE) -f docker-compose.prod.yml up -d

.PHONY: docker-down
docker-down: ## Stop all containers
	$(COMPOSE) down

# ── Observability ─────────────────────────────────────────────────────────────

.PHONY: grafana-import
grafana-import: ## Import all Grafana dashboards via API (requires GRAFANA_URL and GRAFANA_TOKEN)
	@[ -n "$(GRAFANA_URL)" ] || (echo "Set GRAFANA_URL=http://localhost:3000"; exit 1)
	@for f in src/observability/dashboards/*.json; do \
	  echo "Importing $$f..."; \
	  curl -s -X POST "$(GRAFANA_URL)/api/dashboards/import" \
	    -H "Content-Type: application/json" \
	    -H "Authorization: Bearer $(GRAFANA_TOKEN)" \
	    -d "{\"dashboard\": $$(cat $$f), \"overwrite\": true, \"folderId\": 0}" \
	    | jq -r '.status // .message'; \
	done

# ── Security ──────────────────────────────────────────────────────────────────

.PHONY: govulncheck
govulncheck: ## Run Go vulnerability scanner
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

.PHONY: gosec
gosec: ## Run gosec static security analyser
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

# ── Utility ───────────────────────────────────────────────────────────────────

.PHONY: env
env: ## Copy .env.example to .env if .env does not exist
	@[ -f .env ] && echo ".env already exists — skipping" || (cp .env.example .env && echo "Created .env from .env.example")

.PHONY: tidy
tidy: ## Run go mod tidy across all modules
	@for mod_dir in src/gateway src/services/core src/services/pricing \
	    src/services/intelligence src/shared src/observability; do \
	  echo "tidy $$mod_dir"; \
	  (cd $$mod_dir && $(GO) mod tidy); \
	done

.PHONY: seed
seed: ## Seed development database (cities + sample rates)
	./scripts/seed.sh

.PHONY: pre-deploy
pre-deploy: ## Run pre-deploy health checks (see scripts/pre_deploy_check.sh)
	./scripts/pre_deploy_check.sh

.PHONY: clean
clean: ## Remove build artifacts and coverage files
	rm -f coverage.out coverage.html
	$(GO) clean ./...
