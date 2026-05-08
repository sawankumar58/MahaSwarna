<div align="center">

<img src="https://img.shields.io/badge/Platform-Android%20Only-3DDC84?style=for-the-badge&logo=android&logoColor=white" />
<img src="https://img.shields.io/badge/Backend-Go%201.23-00ADD8?style=for-the-badge&logo=go&logoColor=white" />
<img src="https://img.shields.io/badge/Database-PostgreSQL%2015-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" />
<img src="https://img.shields.io/badge/Cache-Redis%20Sentinel-DC382D?style=for-the-badge&logo=redis&logoColor=white" />
<img src="https://img.shields.io/badge/AI-Google%20Gemini-4285F4?style=for-the-badge&logo=google&logoColor=white" />

<br/><br/>

# MahaSwarna

### Real-time gold and silver price intelligence for the Indian jewellery trade

*Live rates · Price alerts · Jewellery catalog · Shopkeeper billing · Diary*

<br/>

</div>

---

## Overview

**MahaSwarna** is a production-grade Android application delivering live gold and silver spot prices to jewellers and traders across India. The platform is built for the Indian market from the ground up — 61 cities, IST-aware rate scheduling, Gemini AI as the sole rate source, dual-provider OTP (Firebase + MSG91), and INR-native formatting throughout.

The architecture targets **10,000 DAU** on a single Hetzner CPX41 node (~₹6,000/month), with a documented upgrade path to Kubernetes at 50k DAU.

| Performance Target | Value |
|---|---|
| Cold start — first meaningful UI (from local cache) | **≤ 400ms** |
| WebSocket live rates connected | **1–2 seconds** |
| Pre-launch load test (k6) | 1,200 users · 100 RPS · 750 WS · p95 < 500ms |

---

## Table of Contents

- [Features](#features)
- [System Architecture](#system-architecture)
- [Repositories](#repositories)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Environment Variables](#environment-variables)
- [Testing](#testing)
- [Deployment](#deployment)
- [Documentation](#documentation)
- [Contributing](#contributing)

---

## Features

| Feature | Description |
|---|---|
| **Live Rates** | Gold and silver spot prices for 61 Indian cities, streamed via WebSocket |
| **AI-Powered Pricing** | Gemini generates rates hourly during IST market hours (Mon–Sat, 10:00–19:00) |
| **Price Alerts** | Push notifications when gold or silver crosses a user-defined threshold |
| **Calculator** | Buy/sell calculator with making charges, GST, and INR formatting (lakh/crore) |
| **Jewellery Catalog** | Full-text search and region-aware AI recommendations with offline browse cache |
| **Shopkeeper Billing** | PDF invoice generation with shop branding and rate-source warnings |
| **Diary** | Local-only bills ledger, customer management, and transaction history (FTS4 search) |
| **Marketplace** | Shop registration and S3-backed banner upload with Gemini Vision content moderation |
| **Subscriptions** | Google Play Billing 7 with Play Integrity attestation on login and purchase |

---

## System Architecture

```
┌──────────────────────────────────────────────────┐
│          Android Client  (Kotlin / Compose)               │
│   Hilt · Room · OkHttp 5 · Retrofit 3 · Coil              │
└─────────────────────┬────────────────────────────┘
                      │  HTTPS / WSS
┌─────────────────────▼───────────────────────────┐
│           API Gateway  :4000  (Go / chi)                  │
│  JWT pre-validation · BFF aggregation                     │
│  Rate limiting · Circuit breakers · Abuse detect.         │
└──────┬───────────────┬──────────────┬────────────┘
        │                  │                │
  :4001 core      :4002 pricing   :4003 intelligence
  auth/identity   rates/WS        catalog/marketplace
  billing/IAP     Gemini AI       invoices/shop
  alerts/push     rate watchdog   content moderation
       │               │              │
       └───────────────┴──────────────┘
                          │
          ┌────────────▼─────────────┐
          │       PostgreSQL 15           │
          │   LISTEN/NOTIFY event bus.    │
          └────────────┬─────────────┘
                       │
          ┌────────────▼──────────────────────┐
          │   Redis Sentinel  (3-node)              │
          │   rate cache · JTI revocation           │
          │   WS fanout · session state             │
          └───────────────────────────────────┘
```

### Service Summary

| Service | Port | Responsibilities |
|---|---|---|
| `gateway` | 4000 | Routing, JWT pre-validation, BFF aggregation, rate limiting, feature flags, abuse detection |
| `core` | 4001 | Auth/identity (dual OTP), billing/IAP, price alerts, FCM push, feature flags |
| `pricing` | 4002 | Gold/silver rates, WebSocket fanout, Gemini AI scheduling, rate quality watchdog |
| `intelligence` | 4003 | Jewellery catalog (FTS + AI), marketplace, PDF invoice generation |

> **WebSocket bypass (ADR-002):** Port `:4002` is publicly exposed. Clients connect directly, bypassing gateway middleware. Compensating controls are enforced in `ws_server.go`. See [`ARCHITECTURE.md`](ARCHITECTURE.md) for full rationale.

---

## Repositories

| Repository | Contents |
|---|---|
| `mahaswarna-backend` | Go monorepo (gateway + core + pricing + intelligence), Docker Compose, migrations, scripts |
| `mahaswarna-android` | Kotlin Android app (Jetpack Compose, Hilt, Room, Retrofit 3, OkHttp 5) |

---

## Getting Started

### Prerequisites

| Tool | Required Version |
|---|---|
| Go | 1.23+ |
| Docker + Docker Compose | Latest stable |
| Android Studio | Hedgehog (2023.1) or newer |
| Kotlin | 2.2.20 |
| JDK | 17+ |
| `golang-migrate` CLI | Latest |

### Backend — Local Setup

```bash
# 1. Clone the backend repository
git clone https://github.com/mahaswarna/mahaswarna-backend
cd mahaswarna-backend

# 2. Copy the example env file and fill in required values
cp .env.example .env

# 3. Start all services (Postgres + Redis + 4 Go services)
docker compose up -d

# 4. Run database migrations
bash scripts/migrate.sh --service=core
bash scripts/migrate.sh --service=pricing
bash scripts/migrate.sh --service=intelligence

# 5. Seed development data
bash scripts/seed.sh

# 6. Verify all services are healthy
bash scripts/smoke_test.sh
```

### Android — Local Setup

```bash
# 1. Clone the Android repository
git clone https://github.com/mahaswarna/mahaswarna-android
cd mahaswarna-android

# 2. Open in Android Studio and sync Gradle

# 3. Connect a device or start an emulator

# 4. Run the debug build (targets http://10.0.2.2:4000)
./gradlew installDebug
```

> **Emulator networking:** Use `http://10.0.2.2:4000` for the gateway and `ws://10.0.2.2:4002` for WebSocket in debug builds. `localhost` on the emulator resolves to the emulator itself, not the host machine.

---

## Development Workflow

### Backend

```bash
golangci-lint run             # Lint
go vet ./...                  # Vet
go test ./... -race -cover    # Test with race detector
go build ./...                # Build all services
```

### Android

```bash
./gradlew ktlintCheck detekt  # Lint
./gradlew test                # Unit tests
./gradlew connectedCheck      # Instrumented tests (requires device or emulator)
./gradlew bundleRelease       # Release AAB
```

### Pre-Deploy Gate

Both repositories enforce a pre-deploy gate before CI proceeds to deployment:

```bash
# Backend — runs migration dry-run, env validation, and JWT round-trip
bash scripts/pre_deploy_check.sh

# Android — validates all required production env vars are present
bash scripts/env_config_check.sh
```

---

## Environment Variables

Key variables required in `.env.production`. Full reference in [`RUNBOOK.md`](RUNBOOK.md).

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |
| `REDIS_SENTINEL_1/2/3` | Redis Sentinel node addresses — **launch gate**; single-node Redis is not acceptable |
| `JWT_PRIVATE_KEY` / `JWT_PUBLIC_KEY` | RS256 signing and verification keys |
| `INTERNAL_JWT_SECRET` | Service-to-service HMAC-SHA256 signing (≥ 64 chars) |
| `GEMINI_API_KEY` | Gemini AI rate generation — server-only, never forwarded to clients |
| `OTP_PROVIDER` | `firebase` \| `msg91` \| `both` |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | Firebase Admin SDK for OTP verification |
| `MSG91_AUTH_KEY` | MSG91 DLT-compliant SMS OTP gateway |
| `PLAY_INTEGRITY_DECRYPTION_KEY` | Play Integrity token verification |
| `SENTRY_DSN` | Error tracking |
| `PAGERDUTY_KEY` | Alertmanager → PagerDuty routing |

---

## Testing

### Backend Test Strategy

Tests use the `testing` stdlib with `testcontainers-go` for real Postgres and Redis integration.

| Package | Key Coverage |
|---|---|
| `test/core/` | Login, refresh, account deletion (dual-fire + idempotency compliance — three test cases per `SECURITY.md`), consent idempotency, receipt state machine |
| `test/pricing/` | Rate use cases, AI scheduler, staleness and sanity watchdog |
| `test/intelligence/` | Catalog search, shop registration, invoice generation (all rate source paths) |
| `test/gateway/` | BFF aggregation, circuit breaker, fallback cache, abuse detection |

> **Compliance gate:** `delete_account_usecase_test.go` must exist and cover all three test cases: Test A (user-initiated fire), Test B (system-initiated hard-delete), and Test C (double-fire idempotency). CI fails if this file is absent — it is a compliance invariant, not optional coverage. See `SECURITY.md — Compliance Requirements` for the full specification.

### Android Test Strategy

| Layer | Framework | Coverage |
|---|---|---|
| Unit | JUnit5 + MockK + Turbine | ViewModels, use cases, mappers |
| UI | Compose Testing + Espresso | Screen navigation, shimmer timeout, error states |
| Integration | Room in-memory | Migration safety, Diary row preservation |

---

## Deployment

### Production Stack

| Component | Configuration |
|---|---|
| **VPS** | Hetzner CPX41 (8 vCPU, 16 GB RAM) — validated for 10k DAU |
| **Orchestration** | Docker Compose (`docker-compose.prod.yml`) with memory limits enforced |
| **Database** | PostgreSQL 15 (primary + 1 read replica) |
| **Cache** | Redis Sentinel (3-node: primary + replica + tie-breaker) — **launch gate** |
| **Observability** | Prometheus + Grafana + Loki + Sentry + PagerDuty |

### CI/CD Pipeline

```
Backend — push to main:
  lint → vet → test -race → build → docker build → deploy via SSH

Android — push vX.Y.Z tag:
  lint → test → bundleRelease → sign AAB → upload to Play Store internal track
```

### Pre-Launch Load Test Gate

Before any production release, run the 15-minute k6 scenario:

| Criteria | Target |
|---|---|
| Concurrent users | 1,200 |
| REST throughput | 100 RPS |
| Concurrent WebSocket connections | 750 |
| p95 latency | < 500ms |
| p99 latency | < 2,000ms |
| Error rate | < 0.1% |

---

## Documentation

| Document | Contents |
|---|---|
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | System architecture, service responsibilities, data flows, ADRs, infrastructure sizing |
| [`SECURITY.md`](SECURITY.md) | Security controls, token lifecycle, secret management, compliance requirements |
| [`RUNBOOK.md`](RUNBOOK.md) | Operational procedures: deployment, incident response, secret rotation, backup/restore |

---

## Contributing

1. Branch from `main` using the prefixes `feature/`, `fix/`, or `chore/`
2. All changes require passing lint, vet, and test (with `-race`) before opening a PR
3. Use audit tags in commit messages for tracked fixes: `[AUDIT-N]`, `[NEW-P1-N]`
4. Architecture decisions are recorded as ADRs in `ARCHITECTURE.md` — open a discussion before changing any cross-cutting invariant
5. **Never** add `fallbackToDestructiveMigration()` to the Android Room builder
6. **Never** use `-ktx` Firebase artifacts — the sole exception is `billing-ktx`, which is a documented and intentional exception

---

## License

**Proprietary Software License — MahaSwarna**

Copyright © 2025 MahaSwarna. All Rights Reserved.

This software, including all source code, documentation, assets, architecture specifications, and associated materials (collectively, the "Software"), is the exclusive proprietary property of MahaSwarna and is protected by copyright law and international treaty provisions.

### Contact

For inquiries or report :
**Email:** support@mahaswarna.com · **Website:** https://www.mahaswarna.com

> *Unauthorised use, reproduction, or distribution of this Software may result in severe civil and criminal penalties and will be prosecuted to the maximum extent possible under applicable law.*

--- 
