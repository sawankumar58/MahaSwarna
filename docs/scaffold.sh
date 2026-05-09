#!/usr/bin/env bash
# =============================================================================
# MahaSwarna — Monorepo Scaffold
# Generates the complete backend (Go) + Android (Kotlin) directory structure
# exactly as specified in mahaswarna_backend-architecture.md and
# mahaswarna_frontend-architecture.md
#
# Usage:
#   chmod +x scaffold.sh && bash scaffold.sh
#
# Creates two sibling directories:
#   ./mahaswarna_backend/
#   ./mahaswarna_android/
# =============================================================================
set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${GREEN}[scaffold]${NC} $*"; }
info() { echo -e "${BLUE}[info]${NC}    $*"; }

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
mkf() {
  # mkf <path> [content]
  mkdir -p "$(dirname "$1")"
  if [[ -n "${2-}" ]]; then
    printf '%s\n' "$2" > "$1"
  else
    touch "$1"
  fi
}

go_pkg() {
  # go_pkg <file_path> <package_name>
  mkf "$1" "package ${2}"
}

go_main() {
  # go_main <file_path>
  mkf "$1" "package main

func main() {}
"
}

kt_file() {
  # kt_file <file_path> <package_suffix>  e.g. kt_file Foo.kt core.network
  mkf "$1" "package com.mahaswarna.${2}
"
}

sh_file() {
  mkf "$1" "#!/usr/bin/env bash
set -euo pipefail
# TODO: implement $(basename "$1")
"
  chmod +x "$1"
}

sql_up() {
  # sql_up <file_path> <description>
  mkf "$1" "-- ${2}
-- Migration: $(basename "$1")
-- Direction: UP

-- TODO: implement
"
}

# ---------------------------------------------------------------------------
# ============================================================
#  BACKEND  (Go monorepo)
# ============================================================
# ---------------------------------------------------------------------------
B=mahaswarna_backend
log "Scaffolding backend → ${B}/"

# ── Root files ───────────────────────────────────────────────────────────────
mkf "${B}/.gitignore" "# Binaries
*.exe
*.test

# Env files — never commit production secrets
.env.production
.env.staging

# Go workspace sum
go.work.sum

# IDE
.idea/
.vscode/
*.iml
"

mkf "${B}/.golangci.yml" "run:
  timeout: 5m
  go: '1.23'

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - bodyclose
    - contextcheck
    - noctx
    - rowserrcheck
    - sqlclosecheck

linters-settings:
  goimports:
    local-prefixes: github.com/mahaswarna
"

mkf "${B}/.env.example" "# ── Database ──────────────────────────────────────────────────────────────
DATABASE_URL=postgres://mahaswarna:changeme@localhost:5432/mahaswarna?sslmode=disable

# ── Redis Sentinel (3-node — required launch gate) ────────────────────────
REDIS_SENTINEL_1=redis-sentinel-1:26379
REDIS_SENTINEL_2=redis-sentinel-2:26379
REDIS_SENTINEL_3=redis-sentinel-3:26379

# ── JWT RS256 keys (generate with: openssl genrsa -out jwt.key 2048) ──────
JWT_PRIVATE_KEY=<paste PEM here>
JWT_PUBLIC_KEY=<paste PEM here>

# ── Service-to-service HMAC (≥64 chars) ───────────────────────────────────
INTERNAL_JWT_SECRET=changeme_minimum_64_chars_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# ── OTP providers ─────────────────────────────────────────────────────────
OTP_PROVIDER=both
FIREBASE_PROJECT_ID=
FIREBASE_SERVICE_ACCOUNT_JSON={}
MSG91_AUTH_KEY=
MSG91_TEMPLATE_ID=
MSG91_OTP_EXPIRY_MINUTES=10

# ── Google Play (IAP + Integrity) ─────────────────────────────────────────
GOOGLE_PLAY_PACKAGE_NAME=com.mahaswarna
GOOGLE_SERVICE_ACCOUNT_JSON={}
PLAY_INTEGRITY_DECRYPTION_KEY=

# ── AI ─────────────────────────────────────────────────────────────────────
GEMINI_API_KEY=

# ── Object Storage ────────────────────────────────────────────────────────
S3_BUCKET=mahaswarna-dev
S3_ENDPOINT=http://localhost:9000
CDN_BASE_URL=http://localhost:9000/mahaswarna-dev
KMS_KEY_ARN=

# ── Observability ──────────────────────────────────────────────────────────
SENTRY_DSN=
PAGERDUTY_KEY=
PROMETHEUS_REMOTE_WRITE_URL=

# ── App ────────────────────────────────────────────────────────────────────
APP_ENV=development
"

mkf "${B}/.env.test" "DATABASE_URL=postgres://mahaswarna:test@localhost:5432/mahaswarna_test?sslmode=disable
REDIS_SENTINEL_1=localhost:26379
REDIS_SENTINEL_2=localhost:26380
REDIS_SENTINEL_3=localhost:26381
JWT_PRIVATE_KEY=
JWT_PUBLIC_KEY=
INTERNAL_JWT_SECRET=test_secret_64_chars_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
APP_ENV=test
GEMINI_API_KEY=test
"

# ── go.work ──────────────────────────────────────────────────────────────────
mkf "${B}/go.work" "go 1.23

use (
	./src/contracts
	./src/shared
	./src/infrastructure
	./src/observability
	./src/gateway
	./src/services/core
	./src/services/pricing
	./src/services/intelligence
)
"

# ── Dockerfiles ──────────────────────────────────────────────────────────────
for svc in gateway core pricing intelligence; do
  mkf "${B}/Dockerfile.${svc}" "FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY src/contracts/ ./src/contracts/
COPY src/shared/ ./src/shared/
COPY src/infrastructure/ ./src/infrastructure/
COPY src/observability/ ./src/observability/
COPY src/services/${svc}/ ./src/services/${svc}/
COPY src/gateway/ ./src/gateway/
RUN go build -o /bin/${svc} ./src/services/${svc}/...

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/${svc} /bin/${svc}
ENTRYPOINT [\"/bin/${svc}\"]
"
done

# gateway has a slightly different build path
mkf "${B}/Dockerfile.gateway" "FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY src/contracts/ ./src/contracts/
COPY src/shared/ ./src/shared/
COPY src/infrastructure/ ./src/infrastructure/
COPY src/observability/ ./src/observability/
COPY src/gateway/ ./src/gateway/
RUN go build -o /bin/gateway ./src/gateway/...

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/gateway /bin/gateway
ENTRYPOINT [\"/bin/gateway\"]
"

# ── GitHub Actions ────────────────────────────────────────────────────────────
mkf "${B}/.github/workflows/ci.yml" "name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
      - name: Vet
        run: go vet ./...
      - name: Test (race detector)
        run: go test ./... -race -cover
      - name: Compliance gate
        run: |
          test -f test/core/delete_account_usecase_test.go \\
            || { echo 'FATAL: delete_account_usecase_test.go missing'; exit 1; }
      - name: Build all services
        run: |
          go build ./src/gateway/...
          go build ./src/services/core/...
          go build ./src/services/pricing/...
          go build ./src/services/intelligence/...
"

mkf "${B}/.github/workflows/security_scan.yml" "name: Security Scan
on:
  schedule:
    - cron: '0 9 * * 1'
  workflow_dispatch:

jobs:
  govulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - name: govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...
"

# ── Scripts ──────────────────────────────────────────────────────────────────
for s in migrate seed pre_deploy_check rotate_secrets cleanup_old_data \
          smoke_test warmup_cache backup_postgres restore_postgres \
          env_config_check gen_service_token activate_ws_killswitch setup_firewall; do
  sh_file "${B}/scripts/${s}.sh"
done

# ── Migrations ───────────────────────────────────────────────────────────────
# core
sql_up "${B}/migrations/core/001_create_users.up.sql"          "users table"
mkf    "${B}/migrations/core/001_create_users.down.sql"        "-- DROP TABLE IF EXISTS users;"
sql_up "${B}/migrations/core/002_create_sessions.up.sql"       "sessions / JTI store"
mkf    "${B}/migrations/core/002_create_sessions.down.sql"     "-- DROP TABLE IF EXISTS sessions;"
sql_up "${B}/migrations/core/003_create_consent_log.up.sql"   "consent_log — insert-only"
mkf    "${B}/migrations/core/003_create_consent_log.down.sql" "-- DROP TABLE IF EXISTS consent_log;"
sql_up "${B}/migrations/core/004_create_subscriptions.up.sql"  "subscriptions"
mkf    "${B}/migrations/core/004_create_subscriptions.down.sql" "-- DROP TABLE IF EXISTS subscriptions;"
sql_up "${B}/migrations/core/005_create_receipt_log.up.sql"   "receipt_log — append-only"
mkf    "${B}/migrations/core/005_create_receipt_log.down.sql" "-- DROP TABLE IF EXISTS receipt_log;"
sql_up "${B}/migrations/core/006_create_alerts.up.sql"         "alerts"
mkf    "${B}/migrations/core/006_create_alerts.down.sql"       "-- DROP TABLE IF EXISTS alerts;"
sql_up "${B}/migrations/core/007_create_device_tokens.up.sql"  "device_tokens"
mkf    "${B}/migrations/core/007_create_device_tokens.down.sql" "-- DROP TABLE IF EXISTS device_tokens;"
sql_up "${B}/migrations/core/008_create_feature_flags.up.sql"  "feature_flags + required seed data"
mkf    "${B}/migrations/core/008_create_feature_flags.down.sql" "-- DROP TABLE IF EXISTS feature_flags;"
sql_up "${B}/migrations/core/009_create_flag_audit.up.sql"    "flag_audit"
mkf    "${B}/migrations/core/009_create_flag_audit.down.sql"  "-- DROP TABLE IF EXISTS flag_audit;"
sql_up "${B}/migrations/core/010_create_audit_log.up.sql"     "audit_log — append-only"
mkf    "${B}/migrations/core/010_create_audit_log.down.sql"   "-- DROP TABLE IF EXISTS audit_log;"

# pricing
sql_up "${B}/migrations/pricing/001_create_cities.up.sql"           "cities — 61 city seed"
mkf    "${B}/migrations/pricing/001_create_cities.down.sql"          "-- DROP TABLE IF EXISTS cities;"
sql_up "${B}/migrations/pricing/002_create_gold_rates.up.sql"        "gold_rates"
mkf    "${B}/migrations/pricing/002_create_gold_rates.down.sql"      "-- DROP TABLE IF EXISTS gold_rates;"
sql_up "${B}/migrations/pricing/003_create_ai_rate_snapshots.up.sql" "ai_rate_snapshots (30-day TTL)"
mkf    "${B}/migrations/pricing/003_create_ai_rate_snapshots.down.sql" "-- DROP TABLE IF EXISTS ai_rate_snapshots;"

# intelligence
sql_up "${B}/migrations/intelligence/001_create_design_catalog.up.sql" "design_catalog + tsvector index"
mkf    "${B}/migrations/intelligence/001_create_design_catalog.down.sql" "-- DROP TABLE IF EXISTS design_catalog;"
sql_up "${B}/migrations/intelligence/002_create_shops.up.sql"            "shops"
mkf    "${B}/migrations/intelligence/002_create_shops.down.sql"           "-- DROP TABLE IF EXISTS shops;"
sql_up "${B}/migrations/intelligence/003_create_invoices.up.sql"         "invoices (no pdf_object_key — ADR-001)"
mkf    "${B}/migrations/intelligence/003_create_invoices.down.sql"       "-- DROP TABLE IF EXISTS invoices;"

# ── Tests ─────────────────────────────────────────────────────────────────────
go_pkg "${B}/test/core/login_usecase_test.go"                  "core_test"
go_pkg "${B}/test/core/refresh_usecase_test.go"                "core_test"
go_pkg "${B}/test/core/delete_account_usecase_test.go"         "core_test"  # COMPLIANCE — DO NOT DELETE
go_pkg "${B}/test/core/consent_log_usecase_test.go"            "core_test"
go_pkg "${B}/test/core/verify_receipt_usecase_test.go"         "core_test"
go_pkg "${B}/test/core/evaluate_thresholds_usecase_test.go"    "core_test"
go_pkg "${B}/test/core/deliver_alert_usecase_test.go"          "core_test"
go_pkg "${B}/test/core/flag_usecase_test.go"                   "core_test"
go_pkg "${B}/test/core/hard_delete_job_test.go"                "core_test"

go_pkg "${B}/test/pricing/get_rate_usecase_test.go"            "pricing_test"
go_pkg "${B}/test/pricing/ai_rate_scheduler_test.go"           "pricing_test"
go_pkg "${B}/test/pricing/rate_quality_watchdog_test.go"       "pricing_test"

go_pkg "${B}/test/intelligence/search_design_usecase_test.go"  "intelligence_test"
go_pkg "${B}/test/intelligence/register_shop_usecase_test.go"  "intelligence_test"
go_pkg "${B}/test/intelligence/generate_invoice_usecase_test.go" "intelligence_test"

go_pkg "${B}/test/gateway/bff_aggregator_test.go"              "gateway_test"
go_pkg "${B}/test/gateway/backpressure_test.go"                "gateway_test"
go_pkg "${B}/test/gateway/fallback_cache_test.go"              "gateway_test"
go_pkg "${B}/test/gateway/correlation_headers_test.go"         "gateway_test"
go_pkg "${B}/test/gateway/abuse_detector_test.go"              "gateway_test"

# ── src/contracts ─────────────────────────────────────────────────────────────
mkf "${B}/src/contracts/go.mod" "module github.com/mahaswarna/contracts

go 1.23
"

go_pkg "${B}/src/contracts/events/user_created.go"          "events"
go_pkg "${B}/src/contracts/events/user_banned.go"           "events"
go_pkg "${B}/src/contracts/events/rate_updated.go"          "events"
go_pkg "${B}/src/contracts/events/rate_stale.go"            "events"
go_pkg "${B}/src/contracts/events/subscription_activated.go" "events"
go_pkg "${B}/src/contracts/events/subscription_expired.go"  "events"
go_pkg "${B}/src/contracts/events/alert_delivered.go"       "events"
go_pkg "${B}/src/contracts/events/shop_registered.go"       "events"
go_pkg "${B}/src/contracts/events/flag_updated.go"          "events"
go_pkg "${B}/src/contracts/events/ai_rate_snapshot_ready.go" "events"
go_pkg "${B}/src/contracts/events/account_deleted.go"       "events"

go_pkg "${B}/src/contracts/http/rates_dto.go"      "http"
go_pkg "${B}/src/contracts/http/auth_dto.go"       "http"
go_pkg "${B}/src/contracts/http/billing_dto.go"    "http"
go_pkg "${B}/src/contracts/http/alerts_dto.go"     "http"
go_pkg "${B}/src/contracts/http/shop_dto.go"       "http"
go_pkg "${B}/src/contracts/http/flags_dto.go"      "http"
go_pkg "${B}/src/contracts/http/catalog_dto.go"    "http"
go_pkg "${B}/src/contracts/http/bff_dto.go"        "http"
go_pkg "${B}/src/contracts/http/compliance_dto.go" "http"
go_pkg "${B}/src/contracts/http/invoice_dto.go"    "http"

# ── src/shared ────────────────────────────────────────────────────────────────
mkf "${B}/src/shared/go.mod" "module github.com/mahaswarna/shared

go 1.23
"

go_pkg "${B}/src/shared/logger.go"            "shared"
go_pkg "${B}/src/shared/errors.go"            "shared"
go_pkg "${B}/src/shared/crypto.go"            "shared"
go_pkg "${B}/src/shared/service_token.go"     "shared"
go_pkg "${B}/src/shared/rate_limit_policy.go" "shared"
go_pkg "${B}/src/shared/audit_log.go"         "shared"
go_pkg "${B}/src/shared/types/event_envelope.go" "types"
go_pkg "${B}/src/shared/types/pagination.go"     "types"
go_pkg "${B}/src/shared/types/api_response.go"   "types"

# ── src/infrastructure ────────────────────────────────────────────────────────
mkf "${B}/src/infrastructure/go.mod" "module github.com/mahaswarna/infrastructure

go 1.23

require (
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
)
"

go_pkg "${B}/src/infrastructure/pgnotify/notifier.go" "pgnotify"
go_pkg "${B}/src/infrastructure/pgnotify/listener.go" "pgnotify"
go_pkg "${B}/src/infrastructure/redis/client.go"       "redis"
go_pkg "${B}/src/infrastructure/postgres/pool_factory.go" "postgres"

# ── src/observability ─────────────────────────────────────────────────────────
mkf "${B}/src/observability/go.mod" "module github.com/mahaswarna/observability

go 1.23

require github.com/prometheus/client_golang v1.20.5
"

go_pkg "${B}/src/observability/metrics.go" "observability"
go_pkg "${B}/src/observability/health.go"  "observability"
mkf    "${B}/src/observability/alertmanager/alertmanager.yml" "# TODO: see backend architecture alertmanager section"
mkf    "${B}/src/observability/dashboards/latency_dashboard.json" "{}"
mkf    "${B}/src/observability/dashboards/error_rate_dashboard.json" "{}"
mkf    "${B}/src/observability/dashboards/ws_dashboard.json" "{}"
mkf    "${B}/src/observability/dashboards/abuse_dashboard.json" "{}"
mkf    "${B}/src/observability/alerts/slo_alerts.yaml"   "# TODO: SLO alert rules"
mkf    "${B}/src/observability/alerts/infra_alerts.yaml" "# TODO: infra alert rules"
mkf    "${B}/src/observability/alerts/cost_alerts.yaml"  "# TODO: cost alert rules"

# ── src/gateway ───────────────────────────────────────────────────────────────
mkf "${B}/src/gateway/go.mod" "module github.com/mahaswarna/gateway

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/redis/go-redis/v9 v9.7.0
	github.com/sony/gobreaker v1.0.0
	github.com/segmentio/ksuid v1.0.4
)

replace (
	github.com/mahaswarna/contracts      => ../contracts
	github.com/mahaswarna/infrastructure => ../infrastructure
	github.com/mahaswarna/observability  => ../observability
	github.com/mahaswarna/shared         => ../shared
)
"

go_main "${B}/src/gateway/main.go"
go_pkg  "${B}/src/gateway/router.go"                              "gateway"
go_pkg  "${B}/src/gateway/bff/home_aggregator.go"                 "bff"
go_pkg  "${B}/src/gateway/lib/resilient_proxy.go"                 "lib"
go_pkg  "${B}/src/gateway/lib/fallback_cache.go"                  "lib"
go_pkg  "${B}/src/gateway/lib/retry.go"                           "lib"
go_pkg  "${B}/src/gateway/middleware/request_id.go"               "middleware"
go_pkg  "${B}/src/gateway/middleware/trace_context.go"            "middleware"
go_pkg  "${B}/src/gateway/middleware/version_validator.go"        "middleware"
go_pkg  "${B}/src/gateway/middleware/global_rate_limiter.go"      "middleware"
go_pkg  "${B}/src/gateway/middleware/jwt_pre_validator.go"        "middleware"
go_pkg  "${B}/src/gateway/middleware/feature_flags.go"            "middleware"
go_pkg  "${B}/src/gateway/middleware/service_token_injector.go"   "middleware"
go_pkg  "${B}/src/gateway/middleware/idempotency.go"              "middleware"
go_pkg  "${B}/src/gateway/middleware/ai_quota_interceptor.go"     "middleware"
go_pkg  "${B}/src/gateway/middleware/abuse_detector.go"           "middleware"

# ── src/services/core ─────────────────────────────────────────────────────────
mkf "${B}/src/services/core/go.mod" "module github.com/mahaswarna/core

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
	firebase.google.com/go/v4 v4.14.1
	github.com/robfig/cron/v3 v3.0.1
	golang.org/x/crypto v0.29.0
)

replace (
	github.com/mahaswarna/contracts      => ../../contracts
	github.com/mahaswarna/infrastructure => ../../infrastructure
	github.com/mahaswarna/observability  => ../../observability
	github.com/mahaswarna/shared         => ../../shared
)
"

go_main "${B}/src/services/core/main.go"
go_pkg  "${B}/src/services/core/http/router.go"                                "http"
go_pkg  "${B}/src/services/core/http/auth_handler.go"                         "http"
go_pkg  "${B}/src/services/core/http/compliance_handler.go"                   "http"
go_pkg  "${B}/src/services/core/http/billing_handler.go"                      "http"
go_pkg  "${B}/src/services/core/http/alerts_handler.go"                       "http"
go_pkg  "${B}/src/services/core/http/device_token_handler.go"                 "http"
go_pkg  "${B}/src/services/core/http/flags_handler.go"                        "http"
go_pkg  "${B}/src/services/core/http/internal_handler.go"                     "http"
go_pkg  "${B}/src/services/core/http/middleware/jwt_auth.go"                  "middleware"
go_pkg  "${B}/src/services/core/http/middleware/service_auth.go"              "middleware"
go_pkg  "${B}/src/services/core/http/middleware/rate_limiter.go"              "middleware"
go_pkg  "${B}/src/services/core/domain/user.go"                               "domain"
go_pkg  "${B}/src/services/core/domain/session.go"                            "domain"
go_pkg  "${B}/src/services/core/domain/consent_log.go"                        "domain"
go_pkg  "${B}/src/services/core/domain/subscription.go"                       "domain"
go_pkg  "${B}/src/services/core/domain/payment_state.go"                      "domain"
go_pkg  "${B}/src/services/core/domain/known_skus.go"                         "domain"
go_pkg  "${B}/src/services/core/domain/alert.go"                              "domain"
go_pkg  "${B}/src/services/core/domain/device_token.go"                       "domain"
go_pkg  "${B}/src/services/core/domain/feature_flag.go"                       "domain"
go_pkg  "${B}/src/services/core/application/register_usecase.go"              "application"
go_pkg  "${B}/src/services/core/application/login_usecase.go"                 "application"
go_pkg  "${B}/src/services/core/application/refresh_usecase.go"               "application"
go_pkg  "${B}/src/services/core/application/logout_usecase.go"                "application"
go_pkg  "${B}/src/services/core/application/delete_account_usecase.go"        "application"
go_pkg  "${B}/src/services/core/application/log_consent_usecase.go"           "application"
go_pkg  "${B}/src/services/core/application/verify_receipt_usecase.go"        "application"
go_pkg  "${B}/src/services/core/application/restore_subscription_usecase.go"  "application"
go_pkg  "${B}/src/services/core/application/evaluate_thresholds_usecase.go"   "application"
go_pkg  "${B}/src/services/core/application/deliver_alert_usecase.go"         "application"
go_pkg  "${B}/src/services/core/application/register_device_token_usecase.go" "application"
go_pkg  "${B}/src/services/core/application/set_flag_usecase.go"              "application"
go_pkg  "${B}/src/services/core/infrastructure/user_repository.go"            "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/session_repository.go"         "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/consent_log_repository.go"     "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/subscription_repository.go"    "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/receipt_log_repository.go"     "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/alerts_repository.go"          "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/device_token_repository.go"    "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/flags_repository.go"           "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/audit_log_repository.go"       "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/google_play_client.go"         "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/otp_provider.go"               "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/firebase_otp_provider.go"      "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/msg91_otp_provider.go"         "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/push_notification_client.go"   "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/rate_projection.go"            "infrastructure"
go_pkg  "${B}/src/services/core/infrastructure/db.go"                         "infrastructure"
go_pkg  "${B}/src/services/core/events/notifier.go"                           "events"
go_pkg  "${B}/src/services/core/events/listeners.go"                          "events"
go_pkg  "${B}/src/services/core/jobs/alert_threshold_job.go"                  "jobs"
go_pkg  "${B}/src/services/core/jobs/subscription_expiry_job.go"              "jobs"
go_pkg  "${B}/src/services/core/jobs/hard_delete_job.go"                      "jobs"

# ── src/services/pricing ──────────────────────────────────────────────────────
mkf "${B}/src/services/pricing/go.mod" "module github.com/mahaswarna/pricing

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
	github.com/robfig/cron/v3 v3.0.1
	google.golang.org/api v0.210.0
)

replace (
	github.com/mahaswarna/contracts      => ../../contracts
	github.com/mahaswarna/infrastructure => ../../infrastructure
	github.com/mahaswarna/observability  => ../../observability
	github.com/mahaswarna/shared         => ../../shared
)
"

go_main "${B}/src/services/pricing/main.go"
go_pkg  "${B}/src/services/pricing/http/router.go"                          "http"
go_pkg  "${B}/src/services/pricing/http/rates_handler.go"                   "http"
go_pkg  "${B}/src/services/pricing/http/middleware/service_auth.go"         "middleware"
go_pkg  "${B}/src/services/pricing/http/middleware/rate_limiter.go"         "middleware"
go_pkg  "${B}/src/services/pricing/ws/ws_server.go"                         "ws"
go_pkg  "${B}/src/services/pricing/ws/channel_router.go"                    "ws"
go_pkg  "${B}/src/services/pricing/ws/connection_registry.go"               "ws"
go_pkg  "${B}/src/services/pricing/ws/heartbeat.go"                         "ws"
go_pkg  "${B}/src/services/pricing/ws/ban_service.go"                       "ws"
go_pkg  "${B}/src/services/pricing/ws/redis_fanout.go"                      "ws"
go_pkg  "${B}/src/services/pricing/domain/gold_rate.go"                     "domain"
go_pkg  "${B}/src/services/pricing/domain/city.go"                          "domain"
go_pkg  "${B}/src/services/pricing/domain/ai_rate_snapshot.go"              "domain"
go_pkg  "${B}/src/services/pricing/application/get_rate_usecase.go"         "application"
go_pkg  "${B}/src/services/pricing/application/get_history_usecase.go"      "application"
go_pkg  "${B}/src/services/pricing/application/generate_ai_rates_usecase.go" "application"
go_pkg  "${B}/src/services/pricing/infrastructure/rates_repository.go"      "infrastructure"
go_pkg  "${B}/src/services/pricing/infrastructure/ai_rate_snapshot_repository.go" "infrastructure"
go_pkg  "${B}/src/services/pricing/infrastructure/redis_cache.go"           "infrastructure"
go_pkg  "${B}/src/services/pricing/infrastructure/gemini_client.go"         "infrastructure"
go_pkg  "${B}/src/services/pricing/infrastructure/db.go"                    "infrastructure"
go_pkg  "${B}/src/services/pricing/watchdog/rate_quality_watchdog.go"       "watchdog"
go_pkg  "${B}/src/services/pricing/events/notifier.go"                      "events"
go_pkg  "${B}/src/services/pricing/events/listeners.go"                     "events"
go_pkg  "${B}/src/services/pricing/jobs/ai_rate_scheduler_job.go"           "jobs"

# ── src/services/intelligence ─────────────────────────────────────────────────
mkf "${B}/src/services/intelligence/go.mod" "module github.com/mahaswarna/intelligence

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/aws/aws-sdk-go-v2 v1.32.6
	github.com/signintech/gopdf v0.25.1
	google.golang.org/api v0.210.0
	golang.org/x/text v0.21.0
)

replace (
	github.com/mahaswarna/contracts      => ../../contracts
	github.com/mahaswarna/infrastructure => ../../infrastructure
	github.com/mahaswarna/observability  => ../../observability
	github.com/mahaswarna/shared         => ../../shared
)
"

go_main "${B}/src/services/intelligence/main.go"
go_pkg  "${B}/src/services/intelligence/http/router.go"                               "http"
go_pkg  "${B}/src/services/intelligence/http/catalog_handler.go"                      "http"
go_pkg  "${B}/src/services/intelligence/http/shop_handler.go"                         "http"
go_pkg  "${B}/src/services/intelligence/http/invoice_handler.go"                      "http"
go_pkg  "${B}/src/services/intelligence/http/middleware/service_auth.go"              "middleware"
go_pkg  "${B}/src/services/intelligence/http/middleware/jwt_auth.go"                  "middleware"
go_pkg  "${B}/src/services/intelligence/http/middleware/rate_limiter.go"              "middleware"
go_pkg  "${B}/src/services/intelligence/domain/design.go"                             "domain"
go_pkg  "${B}/src/services/intelligence/domain/shop.go"                               "domain"
go_pkg  "${B}/src/services/intelligence/domain/invoice.go"                            "domain"
go_pkg  "${B}/src/services/intelligence/application/search_design_usecase.go"        "application"
go_pkg  "${B}/src/services/intelligence/application/recommend_design_usecase.go"     "application"
go_pkg  "${B}/src/services/intelligence/application/register_shop_usecase.go"        "application"
go_pkg  "${B}/src/services/intelligence/application/get_banner_upload_url_usecase.go" "application"
go_pkg  "${B}/src/services/intelligence/application/confirm_banner_upload_usecase.go" "application"
go_pkg  "${B}/src/services/intelligence/application/generate_invoice_usecase.go"     "application"
go_pkg  "${B}/src/services/intelligence/infrastructure/design_repository.go"         "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/shop_repository.go"           "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/invoice_repository.go"        "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/invoice_pdf_builder.go"       "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/view_count_cache.go"          "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/s3_client.go"                 "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/moderation_client.go"         "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/cdn_url_builder.go"           "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/subscription_projection.go"   "infrastructure"
go_pkg  "${B}/src/services/intelligence/infrastructure/db.go"                        "infrastructure"
go_pkg  "${B}/src/services/intelligence/jobs/flush_view_counts_job.go"               "jobs"
go_pkg  "${B}/src/services/intelligence/events/notifier.go"                          "events"
go_pkg  "${B}/src/services/intelligence/events/listeners.go"                         "events"

log "Backend scaffold complete ✓"

# ---------------------------------------------------------------------------
# ============================================================
#  ANDROID  (Kotlin / Jetpack Compose)
# ============================================================
# ---------------------------------------------------------------------------
A=mahaswarna_android
PKG=com/mahaswarna
MAIN="${A}/app/src/main/java/${PKG}"

log "Scaffolding Android → ${A}/"

# ── Root ─────────────────────────────────────────────────────────────────────
mkf "${A}/.gitignore" "*.iml
.gradle/
/local.properties
/.idea/
.DS_Store
/build/
/captures/
.externalNativeBuild/
.cxx/
*.jks
*.keystore
google-services.json
"

mkf "${A}/.editorconfig" "root = true

[*]
end_of_line = lf
insert_final_newline = true
charset = utf-8
trim_trailing_whitespace = true

[*.{kt,kts}]
indent_style = space
indent_size = 4
max_line_length = 120
"

mkf "${A}/detekt.yml" "build:
  maxIssues: 0

style:
  MaxLineLength:
    maxLineLength: 120
"

mkf "${A}/ktlint.gradle.kts" "// ktlint configuration — see https://pinterest.github.io/ktlint/
"

mkf "${A}/.github/workflows/ci.yml" "name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-java@v4
        with:
          java-version: '17'
          distribution: 'temurin'
      - uses: gradle/actions/setup-gradle@v4
      - name: Lint
        run: ./gradlew ktlintCheck detekt
      - name: Unit Tests
        run: ./gradlew testDebugUnitTest
      - name: Assemble
        run: ./gradlew assembleDebug
"

mkf "${A}/.github/workflows/release.yml" "name: Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-java@v4
        with:
          java-version: '17'
          distribution: 'temurin'
      - name: Bundle
        run: ./gradlew bundleRelease
      - name: Upload to Play Store
        uses: r0adkll/upload-google-play@v1
        with:
          serviceAccountJsonPlainText: \${{ secrets.PLAY_SERVICE_ACCOUNT_JSON }}
          packageName: com.mahaswarna
          releaseFiles: app/build/outputs/bundle/release/*.aab
          track: internal
"

mkf "${A}/settings.gradle.kts" "pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}
rootProject.name = \"MahaSwarna\"
include(\":app\")
"

mkf "${A}/build.gradle.kts" "// Top-level build file
plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.compose) apply false
    alias(libs.plugins.hilt) apply false
    alias(libs.plugins.ksp) apply false
    alias(libs.plugins.google.services) apply false
    alias(libs.plugins.firebase.crashlytics) apply false
    alias(libs.plugins.kotlin.serialization) apply false
}
"

mkf "${A}/gradle/libs.versions.toml" "[versions]
kotlin            = \"2.2.20\"
ksp               = \"2.2.20-2.0.0\"
agp               = \"8.7.3\"
hilt              = \"2.52\"
room              = \"2.8.3\"
retrofit          = \"3.0.0\"
okhttp            = \"5.0.0-alpha.14\"
coil              = \"3.0.4\"
vico              = \"2.0.0-beta.3\"
firebaseBom       = \"34.0.0\"
billingKtx        = \"7.1.1\"
playIntegrity     = \"1.4.0\"
paging            = \"3.3.5\"
cameraX           = \"1.4.1\"
coroutines        = \"1.9.0\"
serialization     = \"1.7.3\"
datastore         = \"1.1.1\"
securityCrypto    = \"1.1.0-alpha06\"
sentry            = \"7.18.0\"

[plugins]
android-application      = { id = \"com.android.application\",                version.ref = \"agp\" }
kotlin-android           = { id = \"org.jetbrains.kotlin.android\",           version.ref = \"kotlin\" }
kotlin-compose           = { id = \"org.jetbrains.kotlin.plugin.compose\",    version.ref = \"kotlin\" }
kotlin-serialization     = { id = \"org.jetbrains.kotlin.plugin.serialization\", version.ref = \"kotlin\" }
hilt                     = { id = \"com.google.dagger.hilt.android\",         version.ref = \"hilt\" }
ksp                      = { id = \"com.google.devtools.ksp\",                version.ref = \"ksp\" }
google-services          = { id = \"com.google.gms.google-services\",         version = \"4.4.2\" }
firebase-crashlytics     = { id = \"com.google.firebase.crashlytics\",        version = \"3.0.2\" }

[libraries]
# AndroidX
androidx-core-ktx        = { module = \"androidx.core:core-ktx\",                    version = \"1.15.0\" }
androidx-lifecycle-runtime = { module = \"androidx.lifecycle:lifecycle-runtime-ktx\", version = \"2.8.7\" }
androidx-activity-compose  = { module = \"androidx.activity:activity-compose\",       version = \"1.9.3\" }

# Compose
compose-bom              = { module = \"androidx.compose:compose-bom\",              version = \"2024.12.01\" }
compose-ui               = { module = \"androidx.compose.ui:ui\" }
compose-ui-tooling       = { module = \"androidx.compose.ui:ui-tooling\" }
compose-ui-tooling-preview = { module = \"androidx.compose.ui:ui-tooling-preview\" }
compose-material3        = { module = \"androidx.compose.material3:material3\" }

# Hilt
hilt-android             = { module = \"com.google.dagger:hilt-android\",            version.ref = \"hilt\" }
hilt-compiler            = { module = \"com.google.dagger:hilt-android-compiler\",   version.ref = \"hilt\" }
hilt-navigation-compose  = { module = \"androidx.hilt:hilt-navigation-compose\",     version = \"1.2.0\" }

# Room
room-runtime             = { module = \"androidx.room:room-runtime\",                version.ref = \"room\" }
room-ktx                 = { module = \"androidx.room:room-ktx\",                    version.ref = \"room\" }
room-compiler            = { module = \"androidx.room:room-compiler\",               version.ref = \"room\" }
room-paging              = { module = \"androidx.room:room-paging\",                 version.ref = \"room\" }

# Networking
retrofit                 = { module = \"com.squareup.retrofit2:retrofit\",           version.ref = \"retrofit\" }
okhttp-android           = { module = \"com.squareup.okhttp3:okhttp-android\",       version.ref = \"okhttp\" }
okhttp-logging           = { module = \"com.squareup.okhttp3:logging-interceptor\",  version.ref = \"okhttp\" }
kotlinx-serialization-json = { module = \"org.jetbrains.kotlinx:kotlinx-serialization-json\", version.ref = \"serialization\" }
kotlinx-serialization-converter = { module = \"com.jakewharton.retrofit:retrofit2-kotlinx-serialization-converter\", version = \"1.0.0\" }

# Firebase (BOM 34 — NO -ktx suffix except billing)
firebase-bom             = { module = \"com.google.firebase:firebase-bom\",          version.ref = \"firebaseBom\" }
firebase-analytics       = { module = \"com.google.firebase:firebase-analytics\" }
firebase-crashlytics     = { module = \"com.google.firebase:firebase-crashlytics\" }
firebase-messaging       = { module = \"com.google.firebase:firebase-messaging\" }
firebase-auth            = { module = \"com.google.firebase:firebase-auth\" }
firebase-perf            = { module = \"com.google.firebase:firebase-perf\" }

# Billing (-ktx IS intentional — required for coroutine suspend API)
billing-ktx              = { module = \"com.android.billingclient:billing-ktx\",     version.ref = \"billingKtx\" }
play-integrity           = { module = \"com.google.android.play:integrity\",         version.ref = \"playIntegrity\" }

# Paging
paging-runtime           = { module = \"androidx.paging:paging-runtime-ktx\",       version.ref = \"paging\" }
paging-compose           = { module = \"androidx.paging:paging-compose\",           version.ref = \"paging\" }

# Images / Charts
coil-compose             = { module = \"io.coil-kt.coil3:coil-compose\",            version.ref = \"coil\" }
coil-network-okhttp      = { module = \"io.coil-kt.coil3:coil-network-okhttp\",     version.ref = \"coil\" }
vico-compose-m3          = { module = \"com.patrykandpatrick.vico:compose-m3\",     version.ref = \"vico\" }

# Camera
camerax-core             = { module = \"androidx.camera:camera-core\",              version.ref = \"cameraX\" }
camerax-camera2          = { module = \"androidx.camera:camera-camera2\",           version.ref = \"cameraX\" }
camerax-lifecycle        = { module = \"androidx.camera:camera-lifecycle\",         version.ref = \"cameraX\" }
camerax-view             = { module = \"androidx.camera:camera-view\",              version.ref = \"cameraX\" }

# Storage / Prefs
datastore-preferences    = { module = \"androidx.datastore:datastore-preferences\",  version.ref = \"datastore\" }
security-crypto          = { module = \"androidx.security:security-crypto\",         version.ref = \"securityCrypto\" }

# Coroutines
kotlinx-coroutines-android = { module = \"org.jetbrains.kotlinx:kotlinx-coroutines-android\", version.ref = \"coroutines\" }

# Sentry
sentry-android           = { module = \"io.sentry:sentry-android\",                 version.ref = \"sentry\" }

# Navigation
navigation-compose       = { module = \"androidx.navigation:navigation-compose\",   version = \"2.8.4\" }

# Test
junit                    = { module = \"junit:junit\",                              version = \"4.13.2\" }
kotlin-test              = { module = \"org.jetbrains.kotlin:kotlin-test\" }
coroutines-test          = { module = \"org.jetbrains.kotlinx:kotlinx-coroutines-test\", version.ref = \"coroutines\" }
arch-testing             = { module = \"androidx.arch.core:core-testing\",          version = \"2.2.0\" }
"

mkf "${A}/gradle/wrapper/gradle-wrapper.properties" "distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\\://services.gradle.org/distributions/gradle-8.10.2-bin.zip
validateDistributionUrl=true
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
"

mkf "${A}/app/build.gradle.kts" "plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.hilt)
    alias(libs.plugins.ksp)
    alias(libs.plugins.google.services)
    alias(libs.plugins.firebase.crashlytics)
}

android {
    namespace = \"com.mahaswarna\"
    compileSdk = 35

    defaultConfig {
        applicationId = \"com.mahaswarna\"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = \"1.0.0\"
    }

    buildTypes {
        debug {
            buildConfigField(\"String\", \"BASE_URL\", \"\\\"http://10.0.2.2:4000/v1/\\\"\")
            buildConfigField(\"String\", \"WS_URL\",   \"\\\"ws://10.0.2.2:4002\\\"\")
        }
        create(\"staging\") {
            buildConfigField(\"String\", \"BASE_URL\", \"\\\"https://staging-api.mahaswarna.com/v1/\\\"\")
            buildConfigField(\"String\", \"WS_URL\",   \"\\\"wss://staging-ws.mahaswarna.com:4002\\\"\")
        }
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile(\"proguard-android-optimize.txt\"), \"proguard-rules.pro\")
            buildConfigField(\"String\", \"BASE_URL\", \"\\\"https://api.mahaswarna.com/v1/\\\"\")
            buildConfigField(\"String\", \"WS_URL\",   \"\\\"wss://ws.mahaswarna.com:4002\\\"\")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = \"17\" }
}

dependencies {
    implementation(platform(libs.compose.bom))
    implementation(platform(libs.firebase.bom))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime)
    implementation(libs.androidx.activity.compose)
    implementation(libs.compose.ui)
    implementation(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)

    implementation(libs.hilt.android)
    implementation(libs.hilt.navigation.compose)
    ksp(libs.hilt.compiler)

    implementation(libs.room.runtime)
    implementation(libs.room.ktx)
    implementation(libs.room.paging)
    ksp(libs.room.compiler)

    implementation(libs.retrofit)
    implementation(libs.okhttp.android)
    implementation(libs.okhttp.logging)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.kotlinx.serialization.converter)

    implementation(libs.firebase.analytics)
    implementation(libs.firebase.crashlytics)
    implementation(libs.firebase.messaging)
    implementation(libs.firebase.auth)
    implementation(libs.firebase.perf)

    implementation(libs.billing.ktx)
    implementation(libs.play.integrity)

    implementation(libs.paging.runtime)
    implementation(libs.paging.compose)

    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)
    implementation(libs.vico.compose.m3)

    implementation(libs.camerax.core)
    implementation(libs.camerax.camera2)
    implementation(libs.camerax.lifecycle)
    implementation(libs.camerax.view)

    implementation(libs.datastore.preferences)
    implementation(libs.security.crypto)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.navigation.compose)
    implementation(libs.sentry.android)

    testImplementation(libs.junit)
    testImplementation(libs.kotlin.test)
    testImplementation(libs.coroutines.test)
    testImplementation(libs.arch.testing)
}
"

mkf "${A}/app/proguard-rules.pro" "# Retrofit + OkHttp
-dontwarn okhttp3.**
-keep class retrofit2.** { *; }

# kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keepclassmembers class kotlinx.serialization.json.** { *** Companion; }

# Firebase
-keep class com.google.firebase.** { *; }

# Hilt / Dagger
-keep class dagger.** { *; }
-keep class javax.inject.** { *; }

# Room
-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *

# Sentry
-keep class io.sentry.** { *; }
"

mkf "${A}/app/src/main/AndroidManifest.xml" '<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <uses-permission android:name="android.permission.CAMERA" />
    <uses-permission android:name="android.permission.VIBRATE" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

    <application
        android:name=".MahaSwarnApplication"
        android:label="MahaSwarna"
        android:networkSecurityConfig="@xml/network_security_config"
        android:theme="@style/Theme.MahaSwarna">

        <activity
            android:name=".MainActivity"
            android:exported="true"
            android:windowSoftInputMode="adjustResize">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>

        <service
            android:name=".notification.MahaSwarnMessagingService"
            android:exported="false">
            <intent-filter>
                <action android:name="com.google.firebase.MESSAGING_EVENT" />
            </intent-filter>
        </service>

        <provider
            android:name="androidx.core.content.FileProvider"
            android:authorities="${applicationId}.fileprovider"
            android:exported="false"
            android:grantUriPermissions="true">
            <meta-data
                android:name="android.support.FILE_PROVIDER_PATHS"
                android:resource="@xml/file_paths" />
        </provider>

    </application>
</manifest>
'

mkf "${A}/app/src/main/res/xml/network_security_config.xml" '<?xml version="1.0" encoding="utf-8"?>
<!-- Permits cleartext to 10.0.2.2 for debug builds only -->
<network-security-config>
    <debug-overrides>
        <trust-anchors>
            <certificates src="user" />
        </trust-anchors>
        <domain includeSubdomains="true">10.0.2.2</domain>
    </debug-overrides>
    <base-config cleartextTrafficPermitted="false">
        <trust-anchors>
            <certificates src="system" />
        </trust-anchors>
    </base-config>
</network-security-config>
'

mkf "${A}/app/src/main/res/xml/file_paths.xml" '<?xml version="1.0" encoding="utf-8"?>
<!-- FileProvider paths for invoice PDF sharing -->
<paths>
    <cache-path name="invoice_pdfs" path="invoices/" />
</paths>
'

mkf "${A}/app/src/main/res/values/colors.xml" '<?xml version="1.0" encoding="utf-8"?>
<resources>
    <!-- TODO: define gold luxury colour palette -->
    <color name="gold_primary">#B8860B</color>
    <color name="gold_on_primary">#FFFFFF</color>
</resources>
'

mkf "${A}/app/src/main/res/values/strings.xml" '<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">MahaSwarna</string>
    <!-- Calculator -->
    <string name="calculator_gst_label_buy">GST (% if registered supplier)</string>
    <string name="calculator_gst_hint_buy">Enter 3% if buying from GST-registered supplier</string>
</resources>
'

# ── Kotlin source files ───────────────────────────────────────────────────────
kt_file "${MAIN}/MahaSwarnApplication.kt"  "app"
kt_file "${MAIN}/MainActivity.kt"           "app"

# core/network
kt_file "${MAIN}/core/network/ApiConstants.kt"           "core.network"
kt_file "${MAIN}/core/network/RetrofitClient.kt"         "core.network"
kt_file "${MAIN}/core/network/VersionInterceptor.kt"     "core.network"
kt_file "${MAIN}/core/network/AuthInterceptor.kt"        "core.network"
kt_file "${MAIN}/core/network/AiQuotaInterceptor.kt"     "core.network"
kt_file "${MAIN}/core/network/LogRedactionInterceptor.kt" "core.network"
kt_file "${MAIN}/core/network/ApiErrorMapper.kt"         "core.network"

# core/auth
kt_file "${MAIN}/core/auth/SessionManager.kt" "core.auth"
kt_file "${MAIN}/core/auth/TokenStore.kt"     "core.auth"

# core/websocket
kt_file "${MAIN}/core/websocket/WsClient.kt"   "core.websocket"
kt_file "${MAIN}/core/websocket/WsEnvelope.kt" "core.websocket"

# core/di
kt_file "${MAIN}/core/di/NetworkModule.kt"  "core.di"
kt_file "${MAIN}/core/di/DatabaseModule.kt" "core.di"
kt_file "${MAIN}/core/di/WsModule.kt"       "core.di"

# core/storage
kt_file "${MAIN}/core/storage/PreferenceStore.kt" "core.storage"

# core/util
kt_file "${MAIN}/core/util/InrFormatter.kt"            "core.util"
kt_file "${MAIN}/core/util/NotificationChannelSetup.kt" "core.util"

# feature/auth
kt_file "${MAIN}/feature/auth/data/remote/AuthApi.kt"      "feature.auth.data.remote"
kt_file "${MAIN}/feature/auth/data/AuthRepository.kt"      "feature.auth.data"
kt_file "${MAIN}/feature/auth/domain/LoginUseCase.kt"      "feature.auth.domain"
kt_file "${MAIN}/feature/auth/domain/RefreshTokenUseCase.kt" "feature.auth.domain"
kt_file "${MAIN}/feature/auth/ui/PhoneEntryScreen.kt"      "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/OtpScreen.kt"             "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/SplashScreen.kt"          "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/ConsentScreen.kt"         "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/UpdateRequiredScreen.kt"  "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/AuthViewModel.kt"         "feature.auth.ui"
kt_file "${MAIN}/feature/auth/ui/LoginViewModel.kt"        "feature.auth.ui"

# feature/rates
kt_file "${MAIN}/feature/rates/data/remote/RatesApi.kt"         "feature.rates.data.remote"
kt_file "${MAIN}/feature/rates/data/local/RatesDao.kt"          "feature.rates.data.local"
kt_file "${MAIN}/feature/rates/data/RatesRemoteDataSource.kt"   "feature.rates.data"
kt_file "${MAIN}/feature/rates/data/RatesRepository.kt"         "feature.rates.data"
kt_file "${MAIN}/feature/rates/domain/Rate.kt"                  "feature.rates.domain"
kt_file "${MAIN}/feature/rates/domain/GetRateUseCase.kt"        "feature.rates.domain"
kt_file "${MAIN}/feature/rates/ui/RatesDashboardScreen.kt"      "feature.rates.ui"
kt_file "${MAIN}/feature/rates/ui/RatesDashboardViewModel.kt"   "feature.rates.ui"
kt_file "${MAIN}/feature/rates/ui/RateHistoryScreen.kt"         "feature.rates.ui"
kt_file "${MAIN}/feature/rates/ui/RateHistoryViewModel.kt"      "feature.rates.ui"
kt_file "${MAIN}/feature/rates/ui/CityPickerBottomSheet.kt"     "feature.rates.ui"

# feature/calculator
kt_file "${MAIN}/feature/calculator/domain/CalculatorMode.kt"   "feature.calculator.domain"
kt_file "${MAIN}/feature/calculator/ui/CalculatorScreen.kt"     "feature.calculator.ui"
kt_file "${MAIN}/feature/calculator/ui/CalculatorViewModel.kt"  "feature.calculator.ui"

# feature/home
kt_file "${MAIN}/feature/home/data/BffApi.kt"                   "feature.home.data"
kt_file "${MAIN}/feature/home/data/HomeRepository.kt"           "feature.home.data"
kt_file "${MAIN}/feature/home/domain/GetHomeDataUseCase.kt"     "feature.home.domain"
kt_file "${MAIN}/feature/home/ui/HomeScreen.kt"                 "feature.home.ui"
kt_file "${MAIN}/feature/home/ui/HomeViewModel.kt"              "feature.home.ui"

# feature/alerts
kt_file "${MAIN}/feature/alerts/data/AlertsApi.kt"              "feature.alerts.data"
kt_file "${MAIN}/feature/alerts/data/AlertsRepository.kt"       "feature.alerts.data"
kt_file "${MAIN}/feature/alerts/domain/Alert.kt"                "feature.alerts.domain"
kt_file "${MAIN}/feature/alerts/ui/AlertsScreen.kt"             "feature.alerts.ui"
kt_file "${MAIN}/feature/alerts/ui/AlertsViewModel.kt"          "feature.alerts.ui"
kt_file "${MAIN}/feature/alerts/ui/CreateAlertBottomSheet.kt"   "feature.alerts.ui"

# feature/billing
kt_file "${MAIN}/feature/billing/data/BillingApi.kt"                    "feature.billing.data"
kt_file "${MAIN}/feature/billing/data/BillingRepository.kt"             "feature.billing.data"
kt_file "${MAIN}/feature/billing/domain/VerifyReceiptUseCase.kt"        "feature.billing.domain"
kt_file "${MAIN}/feature/billing/domain/RestoreSubscriptionUseCase.kt"  "feature.billing.domain"
kt_file "${MAIN}/feature/billing/integrity/PlayIntegrityVerifier.kt"    "feature.billing.integrity"
kt_file "${MAIN}/feature/billing/ui/PaywallScreen.kt"                   "feature.billing.ui"
kt_file "${MAIN}/feature/billing/ui/PaywallViewModel.kt"                "feature.billing.ui"

# feature/marketplace
kt_file "${MAIN}/feature/marketplace/data/MarketplaceApi.kt"              "feature.marketplace.data"
kt_file "${MAIN}/feature/marketplace/data/MarketplaceRepository.kt"       "feature.marketplace.data"
kt_file "${MAIN}/feature/marketplace/domain/Shop.kt"                      "feature.marketplace.domain"
kt_file "${MAIN}/feature/marketplace/domain/RegisterShopUseCase.kt"       "feature.marketplace.domain"
kt_file "${MAIN}/feature/marketplace/domain/GetBannerUploadUrlUseCase.kt" "feature.marketplace.domain"
kt_file "${MAIN}/feature/marketplace/domain/ConfirmBannerUseCase.kt"      "feature.marketplace.domain"
kt_file "${MAIN}/feature/marketplace/domain/GenerateInvoiceUseCase.kt"    "feature.marketplace.domain"
kt_file "${MAIN}/feature/marketplace/ui/ShopListScreen.kt"                "feature.marketplace.ui"
kt_file "${MAIN}/feature/marketplace/ui/RegisterShopScreen.kt"            "feature.marketplace.ui"
kt_file "${MAIN}/feature/marketplace/ui/ShopBannerScreen.kt"              "feature.marketplace.ui"
kt_file "${MAIN}/feature/marketplace/ui/BillPrintScreen.kt"               "feature.marketplace.ui"
kt_file "${MAIN}/feature/marketplace/ui/ShopViewModel.kt"                 "feature.marketplace.ui"
kt_file "${MAIN}/feature/marketplace/ui/BillPrintViewModel.kt"            "feature.marketplace.ui"

# feature/catalog
kt_file "${MAIN}/feature/catalog/data/CatalogApi.kt"                "feature.catalog.data"
kt_file "${MAIN}/feature/catalog/data/local/CatalogDao.kt"          "feature.catalog.data.local"
kt_file "${MAIN}/feature/catalog/data/CatalogRepository.kt"         "feature.catalog.data"
kt_file "${MAIN}/feature/catalog/domain/Design.kt"                  "feature.catalog.domain"
kt_file "${MAIN}/feature/catalog/domain/SearchDesignUseCase.kt"     "feature.catalog.domain"
kt_file "${MAIN}/feature/catalog/domain/ImageSearchUseCase.kt"      "feature.catalog.domain"
kt_file "${MAIN}/feature/catalog/ui/CatalogScreen.kt"               "feature.catalog.ui"
kt_file "${MAIN}/feature/catalog/ui/CatalogViewModel.kt"            "feature.catalog.ui"
kt_file "${MAIN}/feature/catalog/ui/DesignDetailScreen.kt"          "feature.catalog.ui"
kt_file "${MAIN}/feature/catalog/ui/ImageSearchScreen.kt"           "feature.catalog.ui"

# feature/flags
kt_file "${MAIN}/feature/flags/data/FlagsApi.kt"          "feature.flags.data"
kt_file "${MAIN}/feature/flags/data/FlagsRepository.kt"   "feature.flags.data"
kt_file "${MAIN}/feature/flags/domain/FeatureFlags.kt"    "feature.flags.domain"

# feature/diary
kt_file "${MAIN}/feature/diary/data/local/BillDao.kt"              "feature.diary.data.local"
kt_file "${MAIN}/feature/diary/data/local/LedgerDao.kt"            "feature.diary.data.local"
kt_file "${MAIN}/feature/diary/data/local/CustomerDao.kt"          "feature.diary.data.local"
kt_file "${MAIN}/feature/diary/data/DiaryRepository.kt"            "feature.diary.data"
kt_file "${MAIN}/feature/diary/domain/DiaryBill.kt"                "feature.diary.domain"
kt_file "${MAIN}/feature/diary/domain/LedgerEntry.kt"              "feature.diary.domain"
kt_file "${MAIN}/feature/diary/domain/LedgerSummary.kt"            "feature.diary.domain"
kt_file "${MAIN}/feature/diary/domain/Customer.kt"                 "feature.diary.domain"
kt_file "${MAIN}/feature/diary/domain/AddLedgerEntryUseCase.kt"    "feature.diary.domain"
kt_file "${MAIN}/feature/diary/domain/GetCustomerLedgerUseCase.kt" "feature.diary.domain"
kt_file "${MAIN}/feature/diary/ui/DiaryScreen.kt"                  "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/DiaryViewModel.kt"               "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/LedgerTab.kt"                    "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/CustomersTab.kt"                 "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/CustomerLedgerDetailScreen.kt"   "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/AddLedgerEntryBottomSheet.kt"    "feature.diary.ui"
kt_file "${MAIN}/feature/diary/ui/CustomerDetailScreen.kt"         "feature.diary.ui"

# notification
kt_file "${MAIN}/notification/MahaSwarnMessagingService.kt" "notification"

# local (Room DB)
kt_file "${MAIN}/local/AppDatabase.kt"           "local"
kt_file "${MAIN}/local/dao/RateDao.kt"           "local.dao"
kt_file "${MAIN}/local/dao/HomeDao.kt"           "local.dao"
kt_file "${MAIN}/local/dao/AlertDao.kt"          "local.dao"
kt_file "${MAIN}/local/entity/RateEntity.kt"     "local.entity"
kt_file "${MAIN}/local/entity/HomeEntity.kt"     "local.entity"
kt_file "${MAIN}/local/entity/AlertEntity.kt"    "local.entity"

# remote DTOs
kt_file "${MAIN}/remote/dto/AuthDto.kt"    "remote.dto"
kt_file "${MAIN}/remote/dto/BillingDto.kt" "remote.dto"
kt_file "${MAIN}/remote/dto/AlertDto.kt"   "remote.dto"
kt_file "${MAIN}/remote/dto/ShopDto.kt"    "remote.dto"
kt_file "${MAIN}/remote/dto/RateDto.kt"    "remote.dto"
kt_file "${MAIN}/remote/dto/CatalogDto.kt" "remote.dto"

# mapper
kt_file "${MAIN}/mapper/EntityMapper.kt" "mapper"

# theme
kt_file "${MAIN}/theme/Theme.kt" "theme"
kt_file "${MAIN}/theme/Color.kt" "theme"
kt_file "${MAIN}/theme/Shape.kt" "theme"

# components
kt_file "${MAIN}/components/StaleRateBanner.kt" "components"
kt_file "${MAIN}/components/LoadingShimmer.kt"  "components"
kt_file "${MAIN}/components/ErrorRetryCard.kt"  "components"

# navigation
kt_file "${MAIN}/navigation/NavGraph.kt" "navigation"
kt_file "${MAIN}/navigation/Route.kt"    "navigation"

# test directories
mkdir -p "${A}/app/src/test/java/com/mahaswarna"
mkdir -p "${A}/app/src/androidTest/java/com/mahaswarna"

log "Android scaffold complete ✓"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
BACKEND_FILES=$(find "${B}" -type f | wc -l)
ANDROID_FILES=$(find "${A}" -type f | wc -l)

echo ""
echo "════════════════════════════════════════════════════"
echo " MahaSwarna scaffold complete"
echo "────────────────────────────────────────────────────"
echo " Backend  files : ${BACKEND_FILES}  →  ${B}/"
echo " Android  files : ${ANDROID_FILES}  →  ${A}/"
echo "────────────────────────────────────────────────────"
echo " Next steps:"
echo "  1. cd ${B} && git init && git add . && git commit -m 'chore: scaffold'"
echo "  2. cd ${A} && git init && git add . && git commit -m 'chore: scaffold'"
echo "  3. Generate docker-compose + DB migrations"
echo "════════════════════════════════════════════════════"
