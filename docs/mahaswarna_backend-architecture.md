# MahaSwarna Backend — Complete File Structure
> Architecture for 10k average users. Optimised for fast startup, low operational complexity, and low infra cost (~$60–80/mo). PostgreSQL LISTEN/NOTIFY for async events. 4 services (gateway + core + pricing + intelligence) on Docker Compose (upgrade path to K8s documented). Gemini AI is the sole rate source.
> **Runtime: Go only.** No Rust in this stack.

---

## Table of Contents
- [Tech Stack](#tech-stack)
- [Architecture Overview](#architecture-overview)
- [Infrastructure & Capacity (10k DAU)](#infrastructure--capacity-10k-dau)
- [Migration Path](#migration-path)
- [Root](#root)
- [Contracts](#srccontracts)
- [.github](#github)
- [Scripts](#scripts)
- [Migrations](#migrations)
- [Test](#test)
- [Gateway](#srcgateway)
- [Core](#srcservicescore)
- [Pricing](#srcservicespricing)
- [Intelligence](#srcservicesintelligence)
- [Infrastructure](#srcinfrastructure)
- [Observability](#srcobservability)
- [Shared](#srcshared)

---

## Tech Stack

| Concern | Choice |
|---|---|
| Runtime | Go 1.23 |
| HTTP Framework | Go: `net/http` + `chi` router |
| WebSocket | Go: `gorilla/websocket` — native |
| Auth | JWT (RS256 — asymmetric keys) via `golang-jwt/jwt` |
| OTP Auth | **Dual-provider:** Firebase Authentication (primary) + MSG91 SMS OTP (fallback/secondary). Firebase verifies its own ID tokens server-side via Firebase Admin SDK (Go). MSG91 OTPs are verified via MSG91 REST API (`api.msg91.com/api/v5/otp/verify`). Provider selection is controlled by a feature flag (`otp_provider`: `firebase` \| `msg91` \| `both`). In `both` mode: Firebase is attempted first; on failure or Firebase SDK error, MSG91 is used automatically. Rationale: Firebase covers most Android devices reliably; MSG91 is a Tier-1 Indian SMS gateway with 99.9% SLA and regulatory DLT compliance (TRAI mandated for Indian SMS). |
| Database | PostgreSQL — single instance + 1 read replica via `pgx/v5` |
| Async events | PostgreSQL `LISTEN/NOTIFY` |
| Cache | Redis Sentinel (3-node: primary + replica + tie-breaker) — rate cache, WS fan-out, session TTL, feature flags via `go-redis/v9` `NewFailoverClient` |
| Queue / Jobs | `robfig/cron` + Redis distributed locks |
| IAP Verification | Google Play Developer API |
| Play Integrity | Google Play Integrity API (server-side token verification) |
| AI | Google Gemini API (proxied — key never exposed to client) via `google/generative-ai-go` |
| Object Storage | S3-compatible (shop banners, DB backups) via `aws-sdk-go-v2` |
| Content Moderation | Gemini Vision |
| Monitoring | Prometheus + Grafana via `prometheus/client_golang` |
| Logging | `slog` (structured JSON) → Loki |
| Error Tracking | Sentry via `getsentry/sentry-go` |
| CI | GitHub Actions |
| Container | Docker + docker-compose (dev + prod until ~50k DAU) |
| Secret Management | `.env.production` encrypted at rest; rotate via `rotate_secrets.sh` |
| Mobile | Android only — Kotlin (native) + Retrofit **3.0.0** / OkHttp **5.x** (`okhttp-android` artifact) + WebSocket |
| Service-to-service auth | HMAC-SHA256 shared secret (`INTERNAL_JWT_SECRET`) — all services run on the same Docker bridge network |

> **iOS:** iOS is not supported. All push, IAP, and WebSocket infrastructure is Android/FCM only. The push abstraction in `push_notification_client.go` is the sole integration point to extend for iOS.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────┐
│         Native Mobile Client (MahaSwarna)            │
│         Android: Kotlin / Retrofit 3.0.0 / OkHttp 5.x │
│         WebSocket (OkHttp) · receipt POST            │
└────────────────────────┬─────────────────────────────┘
                         │ HTTPS / WSS
┌────────────────────────▼─────────────────────────────┐
│              API Gateway  (Go / chi)                  │
│  TLS termination · rate limiting · circuit breakers   │
│  JWT pre-validation · feature flags · BFF aggregation │
│  abuse detection · idempotency · service token inject │
└──────┬──────────────┬──────────────┬─────────────────┘
       │              │              │
       ▼              ▼              ▼
   [core]         [pricing]     [intelligence]
   auth/billing   rates/realtime catalog/marketplace
   engagement     WebSocket
   flags
       │              │              │
       └──────────────┴──────────────┘
                      │
             ┌────────▼────────┐
             │   PostgreSQL     │   (per-service schemas)
             │   + LISTEN/NOTIFY│   (async event bus)
             └────────┬────────┘
                      │
             ┌────────▼────────────────┐
             │   Redis Sentinel         │   (3-node: primary + replica + tie-breaker)
             │   cache · pub/sub       │   NewFailoverClient — launch gate
             │   sessions · flags      │   JTI revocation · WS fan-out
             └────────────────────────┘
```

**Service responsibilities:**

| Service | Port | Responsibilities |
|---|---|---|
| gateway | 4000 | routing · JWT pre-validation · rate limiting · BFF aggregation · feature flags · abuse detection |
| core | 4001 | auth/identity · billing/IAP · alerts/engagement · feature flags |
| pricing | 4002 | gold/silver rates · WebSocket realtime · Gemini AI rate scheduling |
| intelligence | 4003 | jewelry catalog (search, recommend, image search) · marketplace · invoices |

---

## Infrastructure & Capacity (10k DAU)

> Validated against: DAU=10k, peak concurrent ~1,200–1,500 (9–11 AM IST burst), peak WS connections ~600–900.

### Load Estimates

| Metric | Value | Basis |
|---|---|---|
| Peak concurrent users | ~1,200–1,500 | 12–15% of DAU online simultaneously |
| Peak concurrent WebSocket connections | ~600–900 | ~60% of concurrent users hold a live WS |
| REST API peak RPS | ~80–120 RPS | 1,200 users × ~4 req/min ÷ 60 |
| BFF `/bff/home` peak RPS | ~25–40 RPS | launch burst during morning open |
| DB concurrent connections needed | ~60–80 | 4 services × 15–20 pool slots each |
| Redis peak ops/sec | ~5,000–8,000 | WS fanout + cache reads + rate limiter |

### VPS Sizing (Hetzner CPX41: 8 vCPU, 16 GB RAM)

| Resource | 10k DAU Load | CPX41 Capacity | Headroom |
|---|---|---|---|
| CPU (all 4 Go services) | ~2–3 vCPU peak | 8 vCPU | 2.5–3× spare |
| RAM (Go services + PG + Redis) | ~6–8 GB | 16 GB | ~2× spare |
| Network (rates + WS) | ~30–50 Mbps | 1 Gbps | 20× spare |
| Disk I/O (PG WAL + Redis RDB) | Low | SSD | Fine |

> Enable swap (4 GB) as a safety valve for Redis OOM events.

### PostgreSQL Connection Pool — Required Config

> **CRITICAL:** pgx defaults to 4 connections if `MaxConns` is not set. Without explicit tuning this will cause connection starvation under load.

Required config per service in `pool_factory.go`:

```go
config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
config.MaxConns          = 20   // adjust per service (see table below)
config.MinConns          = 2
config.MaxConnLifetime   = 30 * time.Minute
config.MaxConnIdleTime   = 5 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute
```

| Service | MaxConns | Notes |
|---|---|---|
| gateway | 5 | Only uses Redis; minimal PG (idempotency log) |
| core | 20 | Auth, billing, alerts — most write-heavy |
| pricing | 15 | Rate reads (mostly Redis), WS state |
| intelligence | 15 | tsvector full-text search + catalog JSONB queries |
| **Total** | **55** | Within PG default `max_connections=100` |

Required `postgresql.conf` (or Docker Compose env):
```
max_connections    = 150
shared_buffers     = 4GB    # 25% of 16 GB RAM
effective_cache_size = 8GB
work_mem           = 16MB
```

### Redis Memory Budget

```
WS connection registry:    ~600 keys × 1 KB  = 0.6 MB
Rate cache (61 cities):    61 × 2 KB × 1 key  = 0.1 MB
BFF shared rate cache:     61 cities × 2 KB   = 0.1 MB   ← city-scoped (not per-user)
BFF alert cache:           ~200 users × 1 KB  = 0.2 MB   ← only users with active alerts (~20% DAU)
Session/JTI revocation:    10,000 × 0.5 KB   = 5.0 MB
Feature flags:             ~10 KB             = trivial
Rate limiter counters:      1,500 × 100B      = 0.2 MB
Catalog rec cache:          50 regions × 5 KB = 0.3 MB
Design view counters:       ~5,000 designs × 50B = 0.25 MB
─────────────────────────────────────────────────────────
Total active data:                            ~6.8 MB     (was ~9 MB with per-user BFF cache)
+ Redis overhead + fragmentation:            ~40 MB peak
+ Sentinel overhead (if co-located):         ~5 MB
```

Redis 2 GB allocation (`maxmemory 2gb`) is well above the actual need.
Confirm `maxmemory-policy allkeys-lru` is set.
Redis Sentinel runs on port 26379 — add to smoke_test.sh assertion.

### High Availability Strategy

At 10k DAU with production jewellers making live pricing decisions, an unplanned VPS restart is a SEV-1. The architecture has client-side graceful degradation (stale cache, `_degraded: true`) but requires server-side redundancy.

**REQUIRED BEFORE GO-LIVE: Redis Sentinel (minimum viable HA)**

Redis is a de-facto SPOF for JTI revocation, rate cache, WS fan-out, and session state.
A Redis crash is an immediate SEV-1 (see Redis Failure Runbook). A single Redis node is
not acceptable for a financial pricing app at any DAU.

Minimum production Redis configuration — Redis Sentinel on 3 nodes:
```
redis-primary:   CX22  (redis-server, primary)
redis-replica:   CX22  (redis-server + sentinel, REPLICAOF primary)
redis-sentinel:  CX11  (sentinel only — tie-breaker for quorum)
```
- go-redis/v9 supports Sentinel natively via `redis.NewFailoverClient()`
- Sentinel quorum=2: automatic failover in ~10–30s on primary failure
- Cost: ~$10–15/mo for the two additional nodes
- client.go (src/infrastructure/redis/) MUST use `NewFailoverClient`, NOT `NewClient`
- Smoke test must assert Sentinel is responding: `redis-cli -p 26379 SENTINEL masters`

This is NOT optional — it is a launch gate.

**Recommended (Option B) for application HA — Hetzner Load Balancer + 2× CPX31 (~$30/mo extra):**

```
LB → CPX31 #1 (active)
   → CPX31 #2 (active, scale-out)
Shared DB: CPX41 dedicated Postgres + Redis Sentinel cluster
RTO: zero (both nodes active)
```

Option A (stepping stone for application tier) — Docker Compose hot standby:
```
Primary VPS (CPX41):  all 4 services
Standby VPS (CX22):   gateway + pricing only (warm standby)
Failover:             manual DNS switch (TTL = 60s)  RTO: ~5 minutes
```
Note: Redis Sentinel is required regardless of which application HA option is chosen.

### Upgrade Triggers (When to Break Out of Single VPS)

| Metric | Threshold | Action |
|---|---|---|
| `pg_stat_activity` wait events (lock/IO) | > 5% of queries | Add PG read replica (already designed) |
| Redis memory usage | > 60% of `maxmemory` | Increase to 4 GB or move to managed Redis |
| Pricing service CPU | > 70% sustained | Extract to dedicated VPS (K8s migration prep) |
| WS concurrent connections | > 5,000 | Shard `connection_registry.go` + add second pricing node |
| DAU | > 50,000 | Full K8s migration as documented below |

---

## Cross-Cutting Invariants

- JWT, receipt tokens, and API keys are **never** written to any log at any level.
- All 5xx errors are captured to Sentry before the response is sent.
- Play Integrity is verified before any purchase-related endpoint executes.
- **Play Integrity on login (required):** A rooted or emulated device that passes OTP verification
  receives a valid JWT and can consume live rates indefinitely on FREE tier — bypassing the purchase
  funnel entirely. `POST /auth/login` MUST require a Play Integrity token in the request body:
  `{ phone, otp, integrityToken }` — verified server-side via `PLAY_INTEGRITY_DECRYPTION_KEY`
  before issuing any JWT. On integrity failure: return `HTTP 403 { "error": "device_not_trusted" }`.
  Client shows a "This device is not supported" blocking screen. The pre-purchase check is retained
  as a second enforcement layer.
- **Play Integrity token expiry on login:** If the integrity token has expired before `POST /auth/login`
  is called (e.g. user took >10 minutes to enter the OTP), Google returns a token-expiry error during
  server-side verification. Return `HTTP 403 { "error": "integrity_token_expired" }`. The client
  surfaces "Session expired — please try again" and resets the login flow to PhoneEntry, forcing the
  user to re-initiate `sendOtp()` and obtain a fresh integrity token. This is distinct from
  `device_not_trusted` — it is a recoverable flow error, not a permanent device block.
- No endpoint trusts client-provided purchase status — the DB subscription record is the only source of truth.
- All environment variables are validated at startup — missing secrets cause a hard crash (`os.Exit(1)`).
- Service-to-service calls use `X-Service-Token: <HMAC-SHA256 of request timestamp + INTERNAL_JWT_SECRET>` — never the user's JWT. All services run on the same Docker bridge network; a shared HMAC secret is the appropriate control at this scale.
- PostgreSQL LISTEN/NOTIFY handles async events. Each service listens on its own channel. Notifications include a JSON payload with event type + entity ID; the listener fetches full data from DB if needed.
- **ADR-002 — WS gateway bypass (DECIDED):** Port `:4002` (pricing/WebSocket) is publicly exposed and clients connect directly, bypassing gateway middleware (rate limiting, JWT pre-validation, abuse detection, circuit breakers). This is a conscious architectural decision, not an oversight.
  Rationale: routing WebSocket upgrades through the gateway adds a second TCP hop and a goroutine per connection in the gateway process. At 600–900 concurrent WS connections this doubles memory overhead without meaningful benefit — the gateway's JWT pre-validator adds no security beyond what `ws_server.go`'s own auth handshake provides.
  Accepted trade-offs and required compensating controls:
    - `ws_server.go` MUST self-enforce: JWT auth handshake, WS handshake rate limit (20 new upgrades/IP/min via Redis key `ws_hs:{ip}`), `http.MaxBytesReader` on the upgrade request body, and TLS-only enforcement (reject plain HTTP in production — see below).
    - Hetzner firewall restricts `:4002` to inbound TCP only (see `setup_firewall.sh`).
    - `smoke_test.sh` asserts plain HTTP to `:4002` returns 403.
  Do not re-route WS through the gateway without profiling memory impact first. If the gateway is ever extracted to a separate VPS, revisit — the hop cost becomes negligible at that point.
  In production, the Hetzner firewall MUST restrict `:4002` to WSS only — plain HTTP requests to `:4002` must be rejected at the firewall level, not just at the application layer.

  **Hetzner Cloud Firewall rules for `:4002` (apply via Hetzner Console or hcloud CLI before go-live):**
  ```bash
  # scripts/setup_firewall.sh — run once during infrastructure provisioning.
  # Restricts :4002 to inbound TCP only (TLS handshake enforced by the application;
  # plain HTTP is rejected by gorilla/websocket upgrade check + application-level TLS requirement).
  # Port :4000 (gateway) is the public REST endpoint — unrestricted inbound TCP.
  # All other service ports (4001, 4003) are NOT exposed to the public internet.

  # Inbound rules:
  hcloud firewall add-rule mahaswarna-prod \
    --direction in --protocol tcp --port 443 --source-ips 0.0.0.0/0,::/0  # HTTPS gateway (via reverse proxy)
  hcloud firewall add-rule mahaswarna-prod \
    --direction in --protocol tcp --port 4000 --source-ips 0.0.0.0/0,::/0  # API gateway (direct, pre-TLS-termination setup)
  hcloud firewall add-rule mahaswarna-prod \
    --direction in --protocol tcp --port 4002 --source-ips 0.0.0.0/0,::/0  # WSS WebSocket
  hcloud firewall add-rule mahaswarna-prod \
    --direction in --protocol tcp --port 22 --source-ips <your-ops-ip>/32   # SSH — restrict to ops IPs only

  # Ports 4001, 4003 are intentionally absent — internal Docker bridge only.

  # Application-level TLS enforcement in ws_server.go (defense-in-depth):
  # ws_server.go checks that the upgrade request came over TLS in production:
  #   if os.Getenv("APP_ENV") == "production" && r.TLS == nil {
  #     http.Error(w, "WSS required", http.StatusForbidden)
  #     return
  #   }
  # This catches misconfigured reverse proxies that forward plain HTTP to :4002.
  ```

  **smoke_test.sh assertion (add to existing smoke test):**
  ```bash
  # Assert plain HTTP to :4002 is rejected (must return 403, not 101 Switching Protocols)
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:4002/ws)
  [[ "$HTTP_CODE" == "403" ]] || { echo "FATAL: plain HTTP accepted on :4002 in production"; exit 1; }
  ```
- **Pre-launch load test gate:** Before go-live, run a 15-minute k6 scenario simulating 1,200 concurrent users (100 RPS REST + 750 concurrent WebSocket connections) against staging. Pass criteria: p95 < 500ms, p99 < 2000ms, error rate < 0.1%. This gate catches pool starvation, Redis contention, and WS fanout regressions before they reach production.

---

## Migration Path

When you outgrow this architecture, extract in this order:

1. **~50k DAU:** Extract `pricing` + WebSocket realtime first — highest traffic, easiest to isolate.
2. **~100k DAU:** Add Kafka when pg NOTIFY becomes a bottleneck (watch `pg_stat_activity` wait events).
3. **Multi-region:** Move to K8s only when you need >3 replicas of any service.
4. The current `src/contracts/` package and per-service schema layout make this migration mechanical.

---

## Root

```
mahaswarna_backend/
├── .env.example
├── .env.test
├── .env.staging
├── .env.production                      # never committed; encrypted at rest
│                                        # SECRET MANAGEMENT POLICY:
│                                        # .env.production is acceptable for initial launch on a
│                                        # single VPS, but has no rotation audit trail, no per-service
│                                        # scoping, and no leak detection. Upgrade path:
│                                        #
│                                        # LAUNCH (now): .env.production encrypted at rest via:
│                                        #   age-encryption (https://age-encryption.org) or
│                                        #   git-crypt with a hardware-backed master key.
│                                        #   Key stored offline (not on the VPS).
│                                        #   rotate_secrets.sh documents all rotation procedures.
│                                        #
│                                        # POST-LAUNCH (~30 days): migrate to HashiCorp Vault OSS
│                                        #   (free, self-hosted on the VPS as a Docker service):
│                                        #   - Per-service Vault policies (core cannot read pricing keys)
│                                        #   - Audit log of every secret read (Vault audit backend → Loki)
│                                        #   - Automatic rotation for INTERNAL_JWT_SECRET and DB passwords
│                                        #   - env_config_check.sh validates Vault is reachable at startup
│                                        #   - Each service reads secrets via VAULT_TOKEN at boot;
│                                        #       token renewal handled by vault-agent sidecar
│                                        #
│                                        # NEVER store secrets in: git history, Docker image layers,
│                                        #   container env vars in docker-compose.yml (use secrets: block),
│                                        #   or Sentry/Grafana logs (log redaction applies to all secrets).
│                                        #
│                                        # LEAK RESPONSE: if any secret is suspected compromised:
│                                        #   1. Rotate immediately via rotate_secrets.sh
│                                        #   2. Revoke all active JTIs (UPDATE sessions SET revoked=true)
│                                        #   3. Notify affected users if JWT_PRIVATE_KEY was exposed
│                                        #   4. Write incident report in docs/incidents/
├── .gitignore
├── .golangci.yml
├── go.work                              # Go workspace (multi-module monorepo)
├── go.work.sum
├── docker-compose.yml                   # dev: all services + postgres + redis
├── docker-compose.prod.yml              # prod: same stack, restart:always, resource limits
│                                        #
│                                        # MEMORY LIMITS (required; without these Linux OOM Killer
│                                        #   will target PostgreSQL on intelligence memory spike):
│                                        #   gateway:      mem_limit: 256m  / mem_reservation: 128m
│                                        #   core:         mem_limit: 512m  / mem_reservation: 256m
│                                        #   pricing:      mem_limit: 768m  / mem_reservation: 384m
│                                        #   intelligence: mem_limit: 1536m / mem_reservation: 512m
│                                        #   postgres:     mem_limit: 8g    / mem_reservation: 6g
│                                        #   redis:        mem_limit: 2g    / mem_reservation: 256m
│                                        #   Total reserved: ~13.5 GB — fits 16 GB node with 2.5 GB OS headroom
│                                        #   OS-level swap (4 GB, fallocate) is a separate safety valve;
│                                        #     Docker containers do NOT use host swap unless --memory-swap set.
│                                        #
│                                        # REDIS SENTINEL CONFIG (required — see HA section):
│                                        #   Three Redis containers required in docker-compose.prod.yml:
│                                        #   redis-primary:
│                                        #     image: redis:7-alpine
│                                        #     command: redis-server --maxmemory 2gb
│                                        #       --maxmemory-policy allkeys-lru
│                                        #       --save 900 1 --save 300 10 --appendonly no
│                                        #   redis-replica:
│                                        #     image: redis:7-alpine
│                                        #     command: redis-server --replicaof redis-primary 6379
│                                        #   redis-sentinel:
│                                        #     image: redis:7-alpine
│                                        #     command: redis-sentinel /etc/redis/sentinel.conf
│                                        #     sentinel.conf must declare quorum=2 and
│                                        #       sentinel monitor mymaster redis-primary 6379 2
│                                        #   smoke_test.sh: redis-cli -p 26379 SENTINEL masters
│                                        #     | grep -q "mymaster"
│                                        #   All services use NewFailoverClient (see redis/client.go).
│                                        #
│                                        # REDIS CONFIG (enforce in compose, not via manual CLI check):
│                                        #   command: redis-server
│                                        #     --maxmemory 2gb
│                                        #     --maxmemory-policy allkeys-lru
│                                        #     --save 900 1
│                                        #     --save 300 10
│                                        #     --appendonly no
│                                        #   smoke_test.sh asserts: redis-cli CONFIG GET maxmemory-policy == allkeys-lru
│                                        #
│                                        # HEALTHCHECKS (use /health/ready, not /health):
│                                        #   /health/ready returns 503 until pg NOTIFY listeners have run
│                                        #   their startup catch-up queries and DB pool is warm.
│                                        #   /health returns 200 as soon as the HTTP server binds —
│                                        #   that is too early; depends_on service_healthy would be unsafe.
│                                        #   postgres healthcheck: pg_isready -U $POSTGRES_USER
│                                        #   redis healthcheck:    redis-cli ping
│                                        #   core healthcheck:     wget -qO- http://localhost:4001/health/ready
│                                        #   pricing healthcheck:  wget -qO- http://localhost:4002/health/ready
│                                        #   intelligence:         wget -qO- http://localhost:4003/health/ready
│                                        #   All non-postgres/redis healthchecks: interval:10s timeout:5s
│                                        #     retries:5 start_period:20s
│                                        #   core: depends_on postgres + redis (service_healthy)
│                                        #   pricing: depends_on postgres + redis + core (service_healthy)
│                                        #   intelligence: depends_on postgres + redis (service_healthy)
│                                        #   gateway: depends_on core + pricing + intelligence (service_healthy)
│                                        #
│                                        # CACHE WARMER SIDECAR (auto-triggers warmup_cache.sh on
│                                        #   every restart, not only on deploy):
│                                        #   service: cache_warmer
│                                        #     image: curlimages/curl:8
│                                        #     restart: "no"          # one-shot; does not loop
│                                        #     depends_on:
│                                        #       pricing: { condition: service_healthy }
│                                        #       redis:   { condition: service_healthy }
│                                        #     command: /bin/sh /scripts/warmup_cache.sh
│                                        #     volumes: [./scripts:/scripts:ro]
│                                        #   This ensures Redis rate keys are always warm after any
│                                        #   pricing restart (deploy OR crash recovery), not only after
│                                        #   a manual deploy run of warmup_cache.sh.
│
├── Dockerfile.gateway
├── Dockerfile.core
├── Dockerfile.pricing
└── Dockerfile.intelligence
```

---

## Architecture Decision Records

### ADR-001 — Invoice PDF Wire Format (DECIDED)

**Status:** Decided — must be implemented exactly as specified on both backend and Android.

**Decision:** Option A — JSON wrapper with base64-encoded PDF bytes.

**Wire format:**
```json
{
  "invoice_id":   "uuid-string",
  "pdf_bytes":    "<base64-encoded PDF>",
  "generated_at": "2025-01-15T10:30:00+05:30",
  "rate_source":  "live | stale | client_override | manual_override"
}
```

**Backend implementation (`intelligence/http/invoice_handler.go`):**
```go
// InvoiceResponse uses []byte — Go's encoding/json automatically base64-encodes []byte fields.
// No manual base64 encoding is required. Return type from generate_invoice_usecase.go
// passes []byte directly; the JSON encoder handles the rest.
type InvoiceResponse struct {
    InvoiceID   string    `json:"invoice_id"`
    PdfBytes    []byte    `json:"pdf_bytes"`   // base64-encoded by encoding/json automatically
    GeneratedAt time.Time `json:"generated_at"`
    RateSource  string    `json:"rate_source"`
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
```

**Android implementation (`data/remote/dto/InvoiceDto.kt`):**
```kotlin
// kotlinx.serialization decodes base64 JSON strings to ByteArray automatically.
@Serializable
data class InvoiceResponse(
    @SerialName("invoice_id")   val invoiceId: String,
    @SerialName("pdf_bytes")    val pdfBytes: ByteArray,   // base64 → ByteArray automatically
    @SerialName("generated_at") val generatedAt: String,
    @SerialName("rate_source")  val rateSource: String
)
// Retrofit return type: suspend fun generateInvoice(...): InvoiceResponse  (NOT ResponseBody)
```

**Rationale:** Simpler than streaming (Option B). PDF invoices at this scale are <500 KB; base64
overhead (~33%) is irrelevant. Single JSON response keeps rateSource, invoiceId, and pdfBytes
in one atomic payload. Switch to Option B only if PDF size exceeds 5 MB and OOM is observed
on budget devices — that threshold is not expected at 10k DAU.

**Scope of change:** Any deviation from this ADR in either codebase is a breaking contract
mismatch. `invoice_handler.go` and `InvoiceDto.kt` are the two canonical locations.
`InvoiceDto.go` in `src/contracts/http/invoice_dto.go` is the single source of truth.

---

## Contracts

```
src/contracts/
├── events/
│   ├── user_created.go                  # UserCreatedV1 struct + pg channel constant
│   ├── user_banned.go
│   ├── rate_updated.go
│   ├── rate_stale.go
│   ├── subscription_activated.go
│   ├── subscription_expired.go
│   ├── alert_delivered.go
│   ├── shop_registered.go
│   ├── flag_updated.go
│   ├── ai_rate_snapshot_ready.go        # { CityID, GoldRate, SilverRate, Source, GeneratedAt }
│   └── account_deleted.go               # { UserID, DeletedAt, RequestedAt }
│
└── http/
    ├── rates_dto.go                     # GetRateRequest, GetRateResponse
    │                                    #   Source string — GAP-04 fix: NOT a closed enum.
    │                                    #   Current sole value: "gemini".
    │                                    #   Future values (e.g. "mcx", "manual_override") extend
    │                                    #   this set without a breaking change. Do NOT model this
    │                                    #   as a Go iota enum or a validated allowlist — new source
    │                                    #   strings must be passable to clients without a backend
    │                                    #   release. The Android client reads Source as a plain
    │                                    #   String and passes it through to rate_viewed analytics.
    │                                    #   GeneratedAt time.Time
    ├── auth_dto.go
    ├── billing_dto.go
    ├── alerts_dto.go
    ├── shop_dto.go
    ├── flags_dto.go
    ├── catalog_dto.go                   # SearchDesignRequest, RecommendRequest,
    │                                    #   DesignResponse, PaginatedDesignResponse
    ├── bff_dto.go                       # HomeResponse (aggregated)
                                         #   HomeResponse struct must include:
                                         #     Degraded bool `json:"_degraded,omitempty"`
                                         #   Set to true by home_aggregator.go when any
                                         #   upstream (pricing or core/alerts) times out and
                                         #   stale cache is served instead. Omitted (false) on
                                         #   full-success responses. Client (BffDto.kt) reads
                                         #   this as a transient delivery signal to show
                                         #   StaleRateBanner; it is NOT persisted to Room.
    ├── compliance_dto.go                # DeleteAccountRequest, ConsentLogRequest
                                         #   ConsentLogRequest struct:
                                         #     UserID      string `json:"user_id"`
                                         #     ConsentType string `json:"consent_type"`
                                         #     Version     string `json:"version"`
                                         #   VALID ConsentType values (enforced by allowlist
                                         #   in log_consent_usecase.go):
                                         #     "privacy_policy" | "tos"
                                         #   "ai_disclaimer" is NOT a valid ConsentType and
                                         #   must never be sent or accepted. Unknown values
                                         #   are rejected with HTTP 400 invalid_consent_type.
    └── invoice_dto.go                   # GenerateInvoiceRequest, InvoiceResponse
                                         #   GenerateInvoiceRequest:
                                         #     ShopID            string
                                         #     CustomerName      string
                                         #     CustomerPhone     string   (optional)
                                         #     Items             []InvoiceLineItem
                                         #       { Description string, MetalType string,
                                         #         WeightGrams float64, RatePerGram float64,
                                         #         MakingCharges float64, Amount float64 }
                                         #     PaymentMode       string   (cash|upi|card)
                                         #     Notes             string   (optional)
                                         #     GoldRateOverride  float64  (optional — client passes
                                         #       live WS rate; server uses this if > 0, skipping
                                         #       its own pricing fetch. Allows invoice generation
                                         #       even when pricing service is degraded.)
                                         #     SilverRateOverride float64 (optional — same pattern)
                                         #   InvoiceResponse:
                                         #     InvoiceID    string   (UUID)
                                         #     PdfBytes     []byte   (raw PDF — client writes to local storage)
                                         #     GeneratedAt  time.Time
                                         #     RateSource   string   ("live"|"stale"|"client_override"|"manual_override")
                                         #       client should warn user if RateSource != "live";
                                         #       unknown future values → treat as "stale" (future-proof)
```

> All request/response structs use Go `encoding/json` with `validate` struct tags (`go-playground/validator`).
> Shared as a Go module imported by all services — single source of truth for wire format.

> **API VERSIONING POLICY (required before first public release):**
> All public routes exposed via the gateway are versioned under `/v1/`. Example: `GET /v1/rates/:cityID`.
> The Android app always sends `Accept-Version: v1` and the gateway rejects requests with an
> unsupported version header with `HTTP 400 { "error": "unsupported_api_version" }`.
>
> BREAKING CHANGE PROCEDURE:
> 1. New contract struct added to `src/contracts/http/` as `*V2` (e.g. `GetRateResponseV2`).
> 2. Gateway routes both `/v1/` and `/v2/` simultaneously during the compatibility window.
> 3. Android app updated in a new release to send `Accept-Version: v2`.
> 4. Compatibility window: `/v1/` maintained for **90 days** after `/v2/` ships.
>    (Play Store update adoption is ~80% within 30 days; 90 days is conservative.)
> 5. After 90 days: `/v1/` returns `HTTP 410 Gone` with `{ "error": "api_version_deprecated",
>    "upgrade_url": "https://play.google.com/store/apps/details?id=com.mahaswarna" }`.
>    App must handle 410 with an "Please update the app" blocking screen.
>
> NON-BREAKING changes (additive fields, new optional request params) do NOT require a version bump.
> Removing or renaming fields, changing field types, or changing response structure ALWAYS require `/v2/`.
>
> DTO naming convention: `rates_dto.go` always contains the current stable version.
> Legacy versions: `rates_dto_v1.go` kept alongside until the deprecation window closes.

---

## .github

```
.github/
└── workflows/
    ├── ci.yml                    # lint → vet → test → build → docker → deploy
    │                             #   Go: golangci-lint + go test ./... -race -cover
    └── security_scan.yml         # Dependabot + govulncheck (weekly)
```

---

## Scripts

```
scripts/
├── migrate.sh                   # --service=core runs migrations for that schema
├── seed.sh                      # Seeds dev DB with fixture data
├── pre_deploy_check.sh          # Migration dry-run, Redis ping, JWT round-trip
├── rotate_secrets.sh            # Rolls INTERNAL_JWT_SECRET with zero-downtime
│                                # ALSO documents RS256 key rotation procedure (JWT_PRIVATE_KEY /
│                                # JWT_PUBLIC_KEY). RS256 rotation procedure:
│                                #
│                                # RS256 KEY ROTATION PROCEDURE (run during low-traffic window):
│                                # 1. Generate new key pair:
│                                #      openssl genrsa -out jwt_new.key 2048
│                                #      openssl rsa -in jwt_new.key -pubout -out jwt_new.pub
│                                # 2. Deploy NEW_JWT_PUBLIC_KEY env var to all services alongside
│                                #    existing JWT_PUBLIC_KEY. Gateway jwt_pre_validator.go and
│                                #    core jwt_auth.go must accept EITHER public key during the
│                                #    grace window. Implement as []rsa.PublicKey tried in order.
│                                # 3. Wait for access token TTL to expire (15 minutes). All tokens
│                                #    signed with the old private key are now expired.
│                                # 4. Rotate JWT_PRIVATE_KEY to the new key. Redeploy all services.
│                                # 5. Remove old public key from NEW_JWT_PUBLIC_KEY fallback list.
│                                # ⚠️  Do NOT rotate in a single step — any in-flight access token
│                                #    signed with the old key will return 401 for up to 15 minutes.
│                                # Key storage: both keys in .env.production (encrypted at rest).
│                                # Review rotation at: key compromise → immediate; routine → 1 year.
├── cleanup_old_data.sh          # Purges flag_audit > 1y, expired sessions
├── smoke_test.sh                # Hits /health/ready on all services; validates JWT + rate read
│                                #   includes WebSocket connect test (ws://localhost:4002)
│                                #   ALSO asserts Redis eviction policy:
│                                #     POLICY=$(redis-cli CONFIG GET maxmemory-policy | tail -1)
│                                #     [[ "$POLICY" == "allkeys-lru" ]] || { echo "FATAL"; exit 1; }
│                                #   ALSO asserts all services report /health/ready 200 (not just /health)
├── warmup_cache.sh              # Run after EVERY deploy AND auto-triggered by cache_warmer sidecar
│                                #   on every container restart (not only on manual deploy).
│                                #   AUTH: GET /rates/:cityID on the pricing service's INTERNAL
│                                #   router (port 4002) is protected by service_auth middleware
│                                #   (validates X-Service-Token HMAC-SHA256 header). It does NOT
│                                #   require a user JWT — the gateway JWT pre-validator only runs
│                                #   on gateway port 4000. Direct calls to pricing:4002 bypass
│                                #   the gateway but must still present X-Service-Token.
│                                #   Generate the token before the warmup loop:
│                                #     SERVICE_TOKEN=$(scripts/gen_service_token.sh)
│                                #   The script MUST use the Docker Compose service name, NOT
│                                #   localhost (the cache_warmer sidecar runs in its own container;
│                                #   localhost:4002 resolves to the sidecar itself, not pricing):
│                                #     BASE_URL="http://pricing:4002"   # Docker bridge DNS
│                                #   NEVER use BASE_URL="http://localhost:4002" in the sidecar.
│                                #   If the warmup must route through the gateway (e.g., for
│                                #   integration testing), the same service token applies:
│                                #     curl -H "X-Service-Token: $SERVICE_TOKEN" http://gateway:4000/v1/rates/$city
│                                #   Covers ALL 61 cities (not just top-10) in parallel curl loop:
│                                #     CITIES=(mumbai delhi kolkata bangalore hyderabad chennai
│                                #       ahmedabad pune jaipur surat lucknow kanpur nagpur
│                                #       visakhapatnam indore thane bhopal patna ludhiana agra
│                                #       rajkot coimbatore vadodara amritsar meerut nashik
│                                #       faridabad ghaziabad aurangabad ranchi howrah jodhpur
│                                #       guwahati chandigarh solapur jabalpur madurai raipur
│                                #       kota kalyan vasai-virar allahabad vijayawada srinagar
│                                #       amravati navi-mumbai pimpri-chinchwad thiruvananthapuram
│                                #       hubli kochi mangalore mysuru hubli-dharwad warangal
│                                #       guntur nellore tirunelveli bhiwandi saharanpur gorakhpur
│                                #       bikaner noida gurgaon)   # all 61 city IDs from cities table
│                                #     for city in "${CITIES[@]}"; do
│                                #       curl -sf "$BASE_URL/rates/$city" \
│                                #         -H "X-Service-Token: $SERVICE_TOKEN" > /dev/null &
│                                #     done; wait
├── backup_postgres.sh           # pg_dump to S3; KMS-encrypted; 7 daily + 4 weekly + 12 monthly
├── restore_postgres.sh          # download → KMS decrypt → pg_restore → smoke test
│                                # RTO CAVEAT: The 30-minute RTO is aspirational.
│                                #   Actual RTO depends on DB size at restore time. At 10k DAU
│                                #   with 30 days of rate snapshots + invoices, the DB may be
│                                #   several GB. Measure actual restore time in a monthly drill
│                                #   against a production-sized backup and update this figure.
│                                #   Until a drill has been completed, treat RTO as UNKNOWN.
│                                #   Required: schedule a restore drill in the first week post-launch.
└── env_config_check.sh          # fail-fast: asserts all required prod env vars present
                                 #   DATABASE_URL, 
                                 #   REDIS_SENTINEL_1, REDIS_SENTINEL_2, REDIS_SENTINEL_3,
                                 #                                #   ← replaces REDIS_URL (single-node)
                                 #                                #   Required for Sentinel failover client
                                 #                                #   (launch gate — see HA section)
                                 #   JWT_PRIVATE_KEY, JWT_PUBLIC_KEY,
                                 #   INTERNAL_JWT_SECRET,         # ← required on ALL 4 services;
                                 #                                #   used for service-to-service HMAC auth.
                                 #                                #   Missing = silent auth bypass between
                                 #                                #   services on the bridge network.
                                 #                                #   Must be ≥64 chars. Validate length here.
                                 #   GEMINI_API_KEY,
                                 #   GOOGLE_PLAY_PACKAGE_NAME, GOOGLE_SERVICE_ACCOUNT_JSON,
                                 #   PLAY_INTEGRITY_DECRYPTION_KEY,
                                 #   S3_BUCKET, S3_ENDPOINT, CDN_BASE_URL,
                                 #   SENTRY_DSN, APP_ENV=production,
                                 #   PAGERDUTY_KEY,              # required; alertmanager
                                 #   PROMETHEUS_REMOTE_WRITE_URL #   silently misconfigures without these
                                 #   OTP_PROVIDER,               # must be firebase|msg91|both
                                 #   FIREBASE_PROJECT_ID,        # required when OTP_PROVIDER != msg91
                                 #   FIREBASE_SERVICE_ACCOUNT_JSON, # required when OTP_PROVIDER != msg91
                                 #   MSG91_AUTH_KEY,             # required when OTP_PROVIDER != firebase
                                 #   MSG91_TEMPLATE_ID,          # required when OTP_PROVIDER != firebase
                                 #   MSG91_OTP_EXPIRY_MINUTES    # default 10 if absent; warn but don't exit
                                 #   iOS vars (APPLE_*) not included (Android-only).
                                 #   exits 1 on ANY missing or malformed var
└── activate_ws_killswitch.sh    # REQUIRED before any WS kill-switch activation (OQ-8 gate).
                                 # Enforces the mandatory two-step sequence that prevents a 429
                                 # storm when clients fall back to REST polling mode.
                                 # MUST be used instead of direct DB updates for kill_switch_ws.
                                 # Script logic:
                                 #   1. Assert current rate_limit_bff_free_rpm == '40' (default).
                                 #      If already raised: warn and skip step 2 (idempotent).
                                 #   2. Raise: UPDATE feature_flags SET value = '60'
                                 #             WHERE key = 'rate_limit_bff_free_rpm';
                                 #   3. Sleep 5 seconds (Redis flag cache TTL is 60s; 5s is
                                 #      sufficient because gateway refreshes flags on pub/sub
                                 #      notification fired by set_flag_usecase.go).
                                 #   4. Flip: UPDATE feature_flags SET value = 'true'
                                 #            WHERE key = 'kill_switch_ws';
                                 #   5. Confirm: SELECT key, value FROM feature_flags
                                 #               WHERE key IN ('kill_switch_ws', 'rate_limit_bff_free_rpm');
                                 #      Print result — operator confirms both values before exiting.
                                 # ALSO used as a reference by generate_ai_rates_usecase.go's
                                 # 3-failure escalation path — that code path calls
                                 # set_flag_usecase.go directly in the correct order:
                                 #   1. SetFlag("rate_limit_bff_free_rpm", "60")
                                 #   2. time.Sleep(5 * time.Second)
                                 #   3. SetFlag("kill_switch_ws", "true")
                                 # The script is the ops-runbook equivalent for manual activation.
                                 # Both paths enforce the same ordering invariant.
```

---

## Migrations

```
migrations/
├── core/
│   ├── 001_create_users.sql
│   ├── 002_create_sessions.sql
│   ├── 003_create_consent_log.sql       # insert-only; REVOKE UPDATE,DELETE from app_role
│   ├── 004_create_subscriptions.sql
│   ├── 005_create_receipt_log.sql       # append-only, immutable
│   ├── 006_create_alerts.sql
│   ├── 007_create_device_tokens.sql
│   ├── 008_create_feature_flags.sql
│   │                                    # includes otp_provider flag (firebase|msg91|both)
│   │                                    # SEED DATA — REQUIRED (insert after CREATE TABLE):
│   │                                    # Feature flags must be seeded on first deploy so that
│   │                                    # GET /config/feature-flags never returns an empty object.
│   │                                    # An empty response causes the Android client to fall back
│   │                                    # to DEFAULT_FLAGS — which is correct — BUT a missing
│   │                                    # kill_switch map causes the client to treat all kill-switches
│   │                                    # as false (disabled), enabling unimplemented features
│   │                                    # (image search) and payment flows on a fresh backend.
│   │                                    # Required seed (INSERT OR IGNORE / ON CONFLICT DO NOTHING):
│   │                                    #
│   │                                    # -- Feature enables (default ON):
│   │                                    # INSERT INTO feature_flags (key, value) VALUES
│   │                                    #   ('ai_enabled',       'true'),
│   │                                    #   ('shop_enabled',     'true'),
│   │                                    #   ('ws_enabled',       'true'),
│   │                                    #   ('payments_enabled', 'true'),
│   │                                    #   ('catalog_enabled',  'true')
│   │                                    # ON CONFLICT (key) DO NOTHING;
│   │                                    #
│   │                                    # -- Kill-switches (default OFF except image_search):
│   │                                    # INSERT INTO feature_flags (key, value) VALUES
│   │                                    #   ('kill_switch_ai',           'false'),
│   │                                    #   ('kill_switch_ws',           'false'),
│   │                                    #   ('kill_switch_payments',     'false'),
│   │                                    #   ('kill_switch_catalog',      'false'),
│   │                                    #   ('kill_switch_image_search', 'true')   ← BLOCKED BY DEFAULT
│   │                                    # ON CONFLICT (key) DO NOTHING;
│   │                                    #
│   │                                    # -- Params:
│   │                                    # INSERT INTO feature_flags (key, value) VALUES
│   │                                    #   ('rate_sanity_threshold_pct', '2.0'),
│   │                                    #   ('otp_provider',              'both'),
│   │                                    #   ('rate_limit_bff_free_rpm',   '40')
│   │                                    # ON CONFLICT (key) DO NOTHING;
│   │                                    #
│   │                                    # ON CONFLICT DO NOTHING ensures re-running migrations
│   │                                    # (e.g. staging reset) does not overwrite operator-changed values.
│   ├── 009_create_flag_audit.sql
│   └── 010_create_audit_log.sql             # actor TEXT, action TEXT, entity TEXT,
│                                            #   entity_id TEXT, metadata JSONB, occurred_at TIMESTAMPTZ
│                                            #   INDEX on (entity, entity_id, occurred_at DESC)
│                                            #   INDEX on actor + occurred_at DESC (compliance queries)
│                                            #   append-only: REVOKE UPDATE, DELETE ON audit_log FROM app_role
│
├── pricing/
│   ├── 001_create_cities.sql
│   ├── 002_create_gold_rates.sql
│   └── 003_create_ai_rate_snapshots.sql
│
└── intelligence/
    ├── 001_create_design_catalog.sql    # tsvector index on title+description+tags
    ├── 002_create_shops.sql
    └── 003_create_invoices.sql          # invoice_id UUID PK, shop_id FK, user_id,
                                         #   customer_name TEXT, customer_phone TEXT,
                                         #   -- NOTE: there is NO customer_id FK and NO
                                         #   -- /customers backend endpoint. Customers are a
                                         #   -- LOCAL Room-only concept in the Android Diary.
                                         #   -- The backend stores customer_name + customer_phone
                                         #   -- as plain denormalised strings. client_id (UUID)
                                         #   -- exists only in BillEntity/CustomerEntity in Room
                                         #   -- and must never be sent to any backend API.
                                         #   items JSONB,
                                         #   subtotal, total, payment_mode, notes,
                                         #   pdf_size_bytes INTEGER,  -- PDF not stored server-side;
                                         #                               size recorded for audit only
                                         #   generated_at
                                         #   INDEX on shop_id + generated_at DESC
```

> Migrations run via `golang-migrate/migrate` CLI.
> Each service owns its schema — cross-schema joins are prohibited.
>
> **ZERO-DOWNTIME MIGRATION POLICY:**
> All schema changes MUST be backward-compatible until the old code version is fully drained.
> Use the expand/contract pattern:
>
> ADDING a column:
>   Step 1 (expand): `ALTER TABLE … ADD COLUMN new_col TYPE DEFAULT NULL` — old code ignores it, safe.
>   Step 2 (deploy): new code reads/writes `new_col`.
>   Step 3 (contract, next release): add NOT NULL constraint or drop DEFAULT if needed.
>
> REMOVING a column:
>   Step 1: new code stops reading/writing the column.
>   Step 2 (deploy + drain old pods).
>   Step 3 (contract): `ALTER TABLE … DROP COLUMN old_col`.
>
> RENAMING a column: treat as add new + copy data + drop old (3 separate deployments).
>
> ROLLBACK: every migration file in `migrations/` MUST have a corresponding `down` migration.
>   `golang-migrate/migrate` supports `migrate -path … down 1` for single-step rollback.
>   Test rollback in staging before every production deploy.
>
> PRE-DEPLOY GATE: `pre_deploy_check.sh` runs `migrate … --dry-run` and validates the migration
> is backward-compatible (no DROP COLUMN, no NOT NULL without DEFAULT, no RENAME in a single step).
> A failing dry-run blocks the CI deploy step.

---

## Test

```
test/
├── core/
│   ├── login_usecase_test.go
│   ├── refresh_usecase_test.go
│   ├── delete_account_usecase_test.go    # CASCADE + AUDIT TRAIL — DAY-1 REQUIRED TEST.
│   │                                     # DUAL-FIRE COVERAGE (both account_deleted fire paths):
│   │                                     #   Test A — user-initiated deletion:
│   │                                     #     call delete_account_usecase.go directly;
│   │                                     #     assert pg NOTIFY account_deleted fired once;
│   │                                     #     assert audit_log entry written with actor=userID.
│   │                                     #   Test B — system-initiated hard-delete (grace period):
│   │                                     #     seed a user with deleted_at < NOW()-30d;
│   │                                     #     run hard_delete_job.go;
│   │                                     #     assert pg NOTIFY account_deleted fired once;
│   │                                     #     assert hard_deleted_at set; assert audit entry written.
│   │                                     #   Test C — double-fire idempotency:
│   │                                     #     fire account_deleted twice for the same userID;
│   │                                     #     assert intelligence listener's DELETE is idempotent
│   │                                     #     (second fire on already-absent rows returns no error).
│   │                                     # Implementation: testcontainers-go (real Postgres).
│   │                                     # This test MUST exist before the first deploy — the dual-fire
│   │                                     # behaviour is a compliance invariant, not a nice-to-have.
│   ├── consent_log_usecase_test.go       # idempotency, immutability
│   ├── verify_receipt_usecase_test.go    # covers PENDING→VERIFIED|FAILED status transitions
│   │                                     # and idempotency guard (INSERT ON CONFLICT DO NOTHING)
│   ├── evaluate_thresholds_usecase_test.go
│   ├── deliver_alert_usecase_test.go
│   ├── flag_usecase_test.go
│   └── hard_delete_job_test.go           # idempotency; audit entry written; rows with
│                                         #   hard_deleted_at set are skipped on re-run
│
├── pricing/
│   ├── get_rate_usecase_test.go
│   ├── ai_rate_scheduler_test.go
│   └── rate_quality_watchdog_test.go    # covers staleness detection AND sanity checks in one suite:
│                                        #   GAP-03 fix: threshold updated from 5% to 2% to match
│                                        #   generate_ai_rates_usecase.go and rate_quality_watchdog.go.
│                                        #   within 2% → pass; >2% → reject + stale;         ← corrected
│                                        #   no prev snapshot → pass (first run);
│                                        #   threshold override via feature flag — test MUST read
│                                        #   the threshold from the flag, not hardcode it, so the
│                                        #   test boundary changes automatically when the flag changes.
│                                        #   Additional required cases: [1.9% delta → pass],
│                                        #   [2.1% delta → reject], [4.9% delta → reject],
│                                        #   [5.1% delta → reject] — the 2–5% band was previously
│                                        #   untested and would silently pass anomalous rates in prod.
│
├── intelligence/
│   ├── search_design_usecase_test.go
│   ├── register_shop_usecase_test.go
│   └── generate_invoice_usecase_test.go  # banner URL injection, line item totals,
│                                         #   PDF bytes non-empty + Content-Type application/pdf;
│                                         #   GoldRateOverride used when > 0;
│                                         #   503 returned when pricing unavailable and
│                                         #     no override provided;
│                                         #   GAP-04 fix: RateSource correct for all FOUR paths: ← corrected
│                                         #     Path 1 — GoldRateOverride > 0
│                                         #               → RateSource == "client_override"
│                                         #     Path 2 — no override, snapshot.Source == "manual_override"
│                                         #               → RateSource == "manual_override"   ← new
│                                         #     Path 3 — no override, snapshot.Stale == true
│                                         #               → RateSource == "stale"
│                                         #     Path 4 — no override, fresh live snapshot
│                                         #               → RateSource == "live"
│                                         #   GAP-H3 fix: BANNER RESIZE ASSERTIONS REQUIRED:
│                                         #     Test A — CDN banner dimensions:
│                                         #       after confirm_banner_upload_usecase runs,
│                                         #       assert stored image width <= 1200 AND height <= 400.
│                                         #     Test B — in-PDF banner dimensions:
│                                         #       parse the generated PDF bytes and extract the embedded
│                                         #       image; assert width <= 600 AND height <= 160.
│                                         #     Test C — in-PDF banner encoding:
│                                         #       assert the embedded banner is JPEG, not PNG or WebP.
│                                         #       Raw PNG/WebP from CDN must never be embedded directly.
│                                         #     Test D — total PDF size:
│                                         #       assert len(pdfBytes) < 500*1024 (500 KB budget).
│                                         #   These tests must use a real banner fixture image (PNG or
│                                         #   WebP, >500 KB) to validate that the resize + re-encode
│                                         #   pipeline actually fires, not just that the paths are
│                                         #   reachable.
│
└── gateway/
    ├── bff_aggregator_test.go            # home aggregation only; cache hit/miss
    ├── backpressure_test.go
    ├── fallback_cache_test.go
    ├── correlation_headers_test.go
    └── abuse_detector_test.go
```

> All Go tests use `testing` stdlib + `testcontainers-go` for Postgres/Redis integration tests.
> Table-driven tests with `t.Run` subtest structure throughout.

---

## Gateway

```
src/gateway/
├── main.go                              # chi bootstrap, initFlagCache(), BFF aggregators
├── router.go                            # Route table → per-service resilient reverse proxies
│                                        #
│                                        # MIDDLEWARE APPLICATION ORDER (required — must match exactly):
│                                        #   r.Use(VersionMiddleware)       // 1st — rejects deprecated/unknown versions
│                                        #   r.Use(JwtPreValidatorMiddleware) // 2nd — validates RS256 token + JTI
│                                        #   r.Use(AiQuotaMiddleware)       // 3rd — reads quota headers after auth
│                                        #   r.Use(LogRedactionMiddleware)  // 4th — strips sensitive values from logs
│                                        # HttpLoggingInterceptor equivalent: optional debug handler, last.
│                                        # RATIONALE: AiQuotaMiddleware must run after JwtPreValidatorMiddleware
│                                        # so it can read user-scoped quota data under a validated identity.
│                                        # If AiQuota runs before JWT validation, unauthenticated requests
│                                        # would reach quota logic — either erroring or applying a default
│                                        # quota to anonymous traffic. VersionMiddleware must be first so
│                                        # deprecated clients are blocked before any token processing.
│                                        # Changing this order is a security regression.
│                                        #
│                                        # GET  /bff/home                    → bff_aggregator.go (inline)
│                                        # --- Auth ---
│                                        # POST /auth/send-otp                → core :4001/auth/send-otp
│                                        # POST /auth/login                   → core :4001/auth/login
│                                        # POST /auth/refresh                 → core :4001/auth/refresh
│                                        # POST /auth/logout                  → core :4001/auth/logout
│                                        # --- Billing ---
│                                        # POST /billing/verify               → core :4001/billing/verify
│                                        #   ⚠️  Previously documented as POST /verify-receipt — INCORRECT.
│                                        #   The canonical public path is /billing/verify (matches BillingApi.kt).
│                                        #   /verify-receipt is NOT a valid public route; do not add it.
│                                        # POST /billing/restore              → core :4001/billing/restore
│                                        # --- Compliance ---
│                                        # DELETE /user/account               → core :4001/user/account
│                                        # POST /user/consent                 → core :4001/user/consent
│                                        #   (all mobile calls go through the gateway; consent
│                                        #    must be routed here — not accessed on core directly)
│                                        # --- Alerts & Engagement ---
│                                        # POST   /alerts                     → core :4001/alerts
│                                        # GET    /alerts                     → core :4001/alerts
│                                        # DELETE /alerts/:id                 → core :4001/alerts/:id
│                                        # POST   /engagement/device-token    → core :4001/engagement/device-token
│                                        # --- Feature Flags ---
│                                        # GET  /config/feature-flags         → core :4001/flags/public
│                                        #   (public client path → rewrites to internal /flags/public;
│                                        #    all client-facing docs must reference /config/feature-flags)
│                                        # --- Catalog ---
│                                        # GET  /catalog/search               → intelligence :4003
│                                        # GET  /catalog/recommend            → intelligence :4003
│                                        # GET  /catalog/designs/:id          → intelligence :4003  ← GAP-01 fix
│                                        #   Required endpoint: increments view_count server-side
│                                        #   (catalog_handler.go IncrViewCount). Client calls this
│                                        #   on DesignDetailScreen entry — omitting the call means
│                                        #   view counts are never incremented (list endpoints do not
│                                        #   call IncrViewCount). See catalog_handler.go.
│                                        # NOTE: POST /catalog/image-search is intentionally ABSENT  ← GAP-02 fix
│                                        #   from this route table. The route stub has been removed.
│                                        #   catalog_handler.go is the canonical source: "DO NOT
│                                        #   IMPLEMENT YET — absent from router.go." Both files now
│                                        #   agree. Add the route only when the full Gemini Vision
│                                        #   pipeline ships alongside killSwitchImageSearch=false.
│                                        # --- Marketplace / Shops ---
│                                        # POST /shops                        → intelligence :4003
│                                        # GET  /shops                        → intelligence :4003
│                                        # POST /shops/:id/banner             → intelligence :4003
│                                        # POST /shops/:id/banner/confirm     → intelligence :4003
│                                        # POST /shops/:id/invoice/generate   → intelligence :4003
│
├── bff/
│   └── home_aggregator.go               # fan-out (errgroup): pricing + core(alerts)
│                                        # SCOPE: rates + alerts ONLY — shop data is intentionally excluded.
│                                        # Clients fetch shop data via GET /shops when ShopBanner is accessed.
│                                        # (Old comment listed intelligence(shops) as a third fan-out target —
│                                        #  that design was superseded by GAP-M7; do not re-add shop data here)
│                                        #
│                                        # BFF CACHE SPLIT — named functions (implement exactly as specified):
│                                        #
│                                        # The BFF response has two distinct components with different cache scopes:
│                                        #   (a) rates   — identical for all users in the same city
│                                        #   (b) alerts  — user-specific (personal price alerts)
│                                        # A single home:{userID} key (old design) wastes Redis at O(users).
│                                        # The split below brings BFF Redis footprint to ~0.3 MB at 10k DAU.
│                                        #
│                                        # func getSharedRates(ctx, rdb, cityID) (*RatesPayload, error)
│                                        #   key: "home:shared:{cityID}"  TTL: 30s
│                                        #   GET → unmarshal → return; miss → fetch from pricing upstream
│                                        #   SET on upstream success (only if non-degraded)
│                                        #
│                                        # func getUserAlerts(ctx, rdb, userID) (*AlertsPayload, error)
│                                        #   key: "home:alerts:{userID}"  TTL: 30s
│                                        #   GET → unmarshal → return; miss → fetch from core upstream
│                                        #   SET on upstream success; SKIP SET if user has zero active alerts
│                                        #   (avoids polluting Redis with empty-alert keys for 80% of users)
│                                        #
│                                        # func buildHomeResponse(rates *RatesPayload, alerts *AlertsPayload) HomeResponse
│                                        #   merges both payloads; alerts may be nil (user has no active alerts).
│                                        #   SCOPE — rates + alerts ONLY (GAP-M7):
│                                        #     Shop data (shop banner, shop details) is NOT included in the
│                                        #     BFF home response. Clients must fetch shop data via a separate
│                                        #     GET /shops call when the ShopBanner feature is accessed.
│                                        #     buildHomeResponse MUST NOT be extended to include shop data
│                                        #     without a separate caching strategy — shop data is user-scoped
│                                        #     and write-heavy; adding it here would degrade BFF latency
│                                        #     and inflate Redis cache memory (each shop record is ~4 KB).
│                                        #
│                                        # func ServeHome(w, r) — main handler:
│                                        #   1. extract cityID from X-Region header (set by jwt_pre_validator)
│                                        #   2. extract userID from JWT sub claim
│                                        #   3. errgroup with 800ms per-upstream context deadline:
│                                        #        rates  := getSharedRates(ctx, rdb, cityID)
│                                        #        alerts := getUserAlerts(ctx, rdb, userID)
│                                        #   4. if any upstream times out → serve stale cache for that component
│                                        #      with _degraded:true on the response; partial responses are valid
│                                        #   5. buildHomeResponse(rates, alerts) → JSON encode → 200
│                                        # Target: BFF responds < 1500ms to unblock first-install shimmer.
│                                        # Cache key total at 10k DAU: 61 city keys + ~2k alert keys = ~0.3 MB.
│
├── lib/
│   ├── resilient_proxy.go               # circuit breaker (gobreaker) + fallback: cachedResponse || degraded stub
│   │                                    # single location for all gobreaker configuration
│   ├── fallback_cache.go                # per-upstream Redis stale cache; write on 2xx, read on breaker open
│   └── retry.go                         # WithRetry() exponential backoff with jitter
│
└── middleware/
    ├── request_id.go                    # inject X-Request-ID (ksuid) + X-Trace-ID (uuid)
    ├── trace_context.go                 # stores { requestID, traceID } in context; propagates downstream
    │                                    #   used for slog log correlation (slog.With("requestID", ...))
    │                                    #   exports ExtractTrace(r) + InjectTrace(headers, ctx)
    ├── version_validator.go             # GAP-05 fix: validates Accept-Version request header.
    │                                    # MUST be registered FIRST in the middleware chain — before
    │                                    # jwt_pre_validator, so a deprecated client is blocked
    │                                    # before any token processing occurs.
    │                                    # KNOWN VERSIONS (update when /v2/ ships):
    │                                    #   supported:  {"v1"}
    │                                    #   deprecated: {} (empty until 90-day /v1/ window closes)
    │                                    #
    │                                    # Logic:
    │                                    #   version := r.Header.Get("Accept-Version")
    │                                    #   if version == "" { version = "v1" }  // missing header = v1
    │                                    #   if _, ok := supported[version]; ok { next(w, r); return }
    │                                    #   if _, ok := deprecated[version]; ok {
    │                                    #     w.WriteHeader(http.StatusGone)          // 410
    │                                    #     json.NewEncoder(w).Encode(map[string]string{
    │                                    #       "error": "api_version_deprecated",
    │                                    #     })
    │                                    #     return
    │                                    #   }
    │                                    #   // unrecognised value (e.g. client bug sending "v3")
    │                                    #   w.WriteHeader(http.StatusBadRequest)      // 400
    │                                    #   json.NewEncoder(w).Encode(map[string]string{
    │                                    #     "error": "unsupported_api_version",
    │                                    #   })
    │                                    #
    │                                    # Client (Android) ApiErrorMapper:
    │                                    #   HTTP 410 (any body)              → ApiError.VersionDeprecated
    │                                    #   HTTP 400 + error=="unsupported_api_version" → ApiError.VersionDeprecated
    │                                    #   Both → non-dismissible UpdateRequiredScreen (deep-link Play Store)
    │                                    # This is the server-side implementation required by PRD §11.
    ├── global_rate_limiter.go           # Redis token bucket; FREE/PREMIUM/ADMIN tiers
    │                                    # 429 includes Retry-After + upgrade hint for FREE tier
    ├── jwt_pre_validator.go             # verify RS256 signature + JTI revocation check
    │                                    # post-validation: inject X-User-Tier from JWT `tier` claim
    │                                    # post-validation: inject X-Region from JWT `region` claim
    │                                    #   resolution: (1) JWT claim `region` if present,
    │                                    #               (2) default "mumbai"
    │                                    #   No IP geolocation library required. City is set at
    │                                    #   registration. Pre-login requests default to "mumbai".
    │                                    # RS256 MULTI-KEY ACCEPTANCE (required for
    │                                    #   zero-downtime key rotation):
    │                                    #   Maintain []rsa.PublicKey tried in order:
    │                                    #     keys := []rsa.PublicKey{primaryKey}
    │                                    #     if newKey != nil { keys = append(keys, newKey) }
    │                                    #   For each incoming JWT, try each key in sequence;
    │                                    #   accept on first successful verification.
    │                                    #   Single-step rotation (swap private key before all
    │                                    #   old-key tokens expire) will 401 in-flight requests.
    │                                    #   See rotate_secrets.sh for the safe 3-step procedure.
    ├── feature_flags.go                 # local Redis cache + pub/sub refresh
    ├── service_token_injector.go        # sign X-Service-Token header:
    │                                    #   HMAC-SHA256(requestTimestamp + INTERNAL_JWT_SECRET)
    │                                    #   all services on the same Docker bridge network
    ├── idempotency.go                   # cache POST responses by Idempotency-Key (Redis 24h TTL)
    │                                    # provides replay protection for payment routes
    ├── ai_quota_interceptor.go          # reads Gemini API usage headers from intelligence responses
    │                                    # and writes per-user AI quota values into response headers
    │                                    # returned to the client.
    │                                    # HEADER CONTRACT (set on all responses from /catalog/* routes
    │                                    # that invoke Gemini — image search, AI recommendations):
    │                                    #   X-Ai-Quota-Used:      <integer>  (requests used this window)
    │                                    #   X-Ai-Quota-Limit:     <integer>  (max requests per window)
    │                                    #   X-Ai-Quota-Reset-At:  <unix_epoch_seconds> (next reset)
    │                                    # Source: Gemini API response headers proxied by intelligence
    │                                    # service and forwarded via X-Internal-Ai-Quota-* headers
    │                                    # to the gateway on the upstream response.
    │                                    # Gateway reads X-Internal-Ai-Quota-* from the upstream
    │                                    # response and emits X-Ai-Quota-* to the client.
    │                                    # Values are NOT sourced from the response body.
    │                                    # Client (Android) reads these response headers via
    │                                    # AiQuotaInterceptor.kt and writes to PreferenceStore:
    │                                    #   aiQuotaUsed, aiQuotaLimit, aiQuotaResetAt.
    │                                    # Intelligence service responsibility:
    │                                    #   After each Gemini call, extract usage from the SDK
    │                                    #   response metadata and set on its HTTP response:
    │                                    #     w.Header().Set("X-Internal-Ai-Quota-Used", strconv.Itoa(used))
    │                                    #     w.Header().Set("X-Internal-Ai-Quota-Limit", strconv.Itoa(limit))
    │                                    #     w.Header().Set("X-Internal-Ai-Quota-Reset-At", strconv.FormatInt(resetAt, 10))
    │                                    # On routes that do NOT call Gemini, headers are omitted;
    │                                    # client retains its last-known quota values from PreferenceStore.
    └── abuse_detector.go                # heuristic abuse detection:
                                         #   - request burst: >100 req/10s same IP → 429
                                         #   - payload probing: repeated 4xx pattern → block 1h
                                         #   all signals written to Redis sorted sets per IP
                                         #   http.MaxBytesReader(w, r.Body, 64*1024) — inlined in
                                         #     router.go middleware chain
                                         #   3s context deadline — inlined in router.go chain;
                                         #     BFF home_aggregator.go uses its own 800ms per-upstream
                                         #     timeout which is the meaningful protection boundary
```

---

## Core

> Port `:4001`. Auth/identity, billing, engagement, and feature flags.
> Single DB schema `core` with logical sub-schemas separated by table prefix.

```
src/services/core/
├── main.go                              # chi bootstrap; starts pg NOTIFY listeners; starts cron jobs
│
├── http/
│   ├── router.go
│   ├── auth_handler.go                  # POST /auth/register — OTP-based; creates user row on first
│   │                                    #   successful OTP verification; idempotent on re-register
│   │                                    #   request body includes cityID (stored as users.city_id;
│   │                                    #   embedded as `region` claim in every issued JWT)
│   │                                    #   GAP-05 fix — ENDPOINT STATUS AND SEMANTICS:
│   │                                    #   POST /auth/register is a LIVE ENDPOINT retained for
│   │                                    #   future non-Android clients (web dashboard, iOS if added).
│   │                                    #   It is NOT deprecated and NOT dead code.
│   │                                    #   The Android v1 client NEVER calls /auth/register —
│   │                                    #   it always uses /auth/login (upsert path handles new
│   │                                    #   users). This is documented in PRD §5.
│   │                                    #   KEY DIFFERENCE vs /auth/login cityID handling:
│   │                                    #   /auth/register: ALWAYS writes cityID to users.city_id
│   │                                    #     (the entire purpose of the register call is to set up
│   │                                    #     the user row; cityID is mandatory and always applied).
│   │                                    #   /auth/login:   writes cityID ONLY on fresh insert
│   │                                    #     (xmax == 0 guard; see login_usecase.go); never
│   │                                    #     overwrites an existing users.city_id.
│   │                                    #   Future clients that call /auth/register must supply
│   │                                    #   cityID; omitting it is a 400 validation error.
│   │                                    #   DEFERRED FCM TOKEN REGISTRATION (G-16):
│   │                                    #   After POST /auth/login issues a JWT and returns the
│   │                                    #   response, the Android client may immediately fire a
│   │                                    #   second request: POST /engagement/device-token with a
│   │                                    #   previously deferred FCM token (one that arrived before
│   │                                    #   the user had logged in). This is a separate request —
│   │                                    #   auth_handler.go does NOT need to handle it; the JWT
│   │                                    #   issued by the login response is sufficient auth for
│   │                                    #   the subsequent device-token call. If POST
│   │                                    #   /engagement/device-token ever adds stricter token-
│   │                                    #   freshness validation, ensure the deferred path still
│   │                                    #   works: the token is registered moments after login,
│   │                                    #   so the JWT will always be fresh at that point.
│   │                                    #   POST /auth/send-otp — triggers OTP delivery
│   │                                    #   request body: { phone: string }
│   │                                    #   Firebase path: no-op (client triggers SMS directly);
│   │                                    #     responds 200 { provider: "firebase" } immediately.
│   │                                    #   MSG91 path: calls Msg91OtpProvider.SendOTP();
│   │                                    #     responds 200 { provider: "msg91" }.
│   │                                    #   "both" mode: responds 200 { provider: "firebase" };
│   │                                    #     client always initiates Firebase flow first.
│   │                                    #   Rate limited: max 5 OTP sends per phone per hour
│   │                                    #     (Redis counter otp_send:{E164phone}, checked before dispatch).
│   │                                    #   POST /auth/login — phone + OTP → JWT pair
│   │                                    #   DUAL-PROVIDER REQUEST BODY:
│   │                                    #     Firebase path: { phone, firebaseIdToken, integrityToken,
│   │                                    #                      cityID?, provider: "firebase" }
│   │                                    #     MSG91 path:    { phone, otp, integrityToken,
│   │                                    #                      cityID?, provider: "msg91" }
│   │                                    #     Provider inferred from field presence if provider absent:
│   │                                    #       firebaseIdToken present → firebase path
│   │                                    #       otp present            → msg91 path
│   │                                    #   SERVER-SIDE SILENT FIREBASE→MSG91 FALLBACK:
│   │                                    #     If firebaseIdToken is present AND the Firebase Admin SDK
│   │                                    #     experiences an infrastructure error (timeout, SDK internal
│   │                                    #     error — NOT a credential verification failure), the server
│   │                                    #     re-attempts via Msg91OtpProvider ONLY if otp is ALSO present
│   │                                    #     in the same request. If otp is absent → return 401 immediately.
│   │                                    #     NOTE: The Android v1 client never sends both firebaseIdToken
│   │                                    #     and otp in the same request body; this path is a server-internal
│   │                                    #     defensive mechanism only — it is NOT triggered by any current
│   │                                    #     client code path. Do not rely on this as a primary auth flow.
│   │                                    #   PLAY INTEGRITY TOKEN EXPIRY:
│   │                                    #     If the integrityToken is expired (user took >~10 min to enter OTP),
│   │                                    #     Google returns an expiry error during verification.
│   │                                    #     Return: HTTP 403 { "error": "integrity_token_expired" }
│   │                                    #     Client shows a retry error and restarts the login flow.
│   │                                    #   VERIFICATION SEQUENCE:
│   │                                    #     1. Play Integrity token verified (required — see invariants)
│   │                                    #     2. OtpProvider.VerifyOTP(phone, token/otp) via dispatcher
│   │                                    #     3. On success: upsert user → issue JWT pair
│   │                                    #     4. On failure: return 401 { error: "otp_invalid" }
│   │                                    #   POST /auth/refresh  — rotate refresh token, revoke old JTI
│   │                                    #   POST /auth/logout   — add JTI to Redis revocation set
│   ├── compliance_handler.go            # DELETE /user/account, POST /user/consent
│   ├── billing_handler.go               # POST /billing/verify  — Google Play receipt verification
│   │                                    # POST /billing/restore — Google Play purchase restore
│   │                                    # (Android-only; no App Store endpoints)
│   ├── alerts_handler.go                # POST /alerts, GET /alerts, DELETE /alerts/:id
│   ├── device_token_handler.go          # POST /engagement/device-token (upsert per userID+deviceID)
│   ├── flags_handler.go                 # GET /flags/public (INTERNAL to core — NOT client-callable)
│   │                                    #   Public gateway path: GET /config/feature-flags
│   │                                    #   (gateway router.go rewrites /config/feature-flags → core:4001/flags/public)
│   │                                    #   All client references must use /config/feature-flags only.
│   │                                    #   JWT-auth, Redis-cached 60s
│   │                                    #   returns: { Flags map[string]bool, KillSwitch map[string]bool,
│   │                                    #     Params map[string]float64 }
│   │                                    #   flags: ai_enabled, shop_enabled, ws_enabled,
│   │                                    #     payments_enabled, catalog_enabled
│   │                                    #   kill_switch: ai, ws, payments, catalog
│   │                                    #   params: rate_sanity_threshold_pct (default 2.0)
│   │                                    #     — read by rate_quality_watchdog.go without deploy;
│   │                                    #     hardcoded fallback in watchdog is also 2.0 (they must match)
│   ├── internal_handler.go              # GET /internal/subscriptions/active
│   │                                    #   Auth: X-Service-Token HMAC-SHA256 only (no JWT).
│   │                                    #   Purpose: allows pricing + intelligence to rebuild their
│   │                                    #   subscription_projection Redis read model on startup and
│   │                                    #   reconnect WITHOUT directly querying core's DB schema.
│   │                                    #   (cross-schema reads are prohibited.)
│   │                                    #   Returns: [{ userID: string, tier: string }]
│   │                                    #   Only active subscriptions (status='active',
│   │                                    #   expires_at > NOW()) are returned.
│   │                                    #   Not exposed externally — gateway has no route to it.
│   │                                    #   Bound only on the Docker bridge network interface.
│   └── middleware/
│       ├── jwt_auth.go
│       ├── service_auth.go              # validates X-Service-Token HMAC-SHA256 header
│       └── rate_limiter.go
│
├── domain/
│   ├── user.go
│   ├── session.go                       # jti, ipAddress
│   ├── consent_log.go                   # UserID, ConsentType, Version, AcceptedAt
│   ├── subscription.go
│   ├── payment_state.go                 # status string: "pending" | "verified" | "failed"
│   │                                    # transitions enforced by verify_receipt_usecase.go
│   │                                    # and a DB CHECK constraint on the receipts table.
│   ├── known_skus.go                    # valid IAP product ID constants
│   ├── alert.go                         # UserID, CityID, Metal, Threshold, Direction
│   ├── device_token.go                  # UserID, DeviceID, Token, Platform, UpdatedAt
│   └── feature_flag.go
│
├── application/
│   ├── register_usecase.go              # validate OTP → upsert user (INSERT ON CONFLICT DO NOTHING)
│   │                                    # → pg NOTIFY user_created → return JWT pair
│   ├── login_usecase.go                 # checkLoginThrottle() → OtpProvider.VerifyOTP()
│   │                                    # → issue JWT pair
│   │                                    # OtpProvider is injected (FirebaseOtpProvider,
│   │                                    #   Msg91OtpProvider, or DualOtpProvider based on flag).
│   │                                    # checkLoginThrottle: max 10 failed login attempts per
│   │                                    #   phone per 15 minutes (Redis counter login_fail:{phone}).
│   │                                    # On OTP verified: reset login_fail counter.
│   │                                    # cityID HANDLING ON LOGIN — GAP-03 fix:
│   │                                    #   cityID is accepted in the request body for client
│   │                                    #   compatibility but MUST NOT update users.city_id for
│   │                                    #   existing (returning) users. The city set at registration
│   │                                    #   is authoritative; overwriting it on every login would
│   │                                    #   silently change the user's rate region.
│   │                                    #   GUARD IMPLEMENTATION (required):
│   │                                    #   Use INSERT ... ON CONFLICT (phone) DO NOTHING
│   │                                    #   RETURNING xmax to detect a fresh insert:
│   │                                    #     INSERT INTO users (phone, city_id, ...)
│   │                                    #     VALUES ($phone, $cityID, ...)
│   │                                    #     ON CONFLICT (phone) DO NOTHING
│   │                                    #     RETURNING xmax
│   │                                    #   xmax == 0  → row was freshly inserted (new user):
│   │                                    #     city_id is already set from the INSERT above. ✓
│   │                                    #   xmax != 0  → conflict occurred (returning user):
│   │                                    #     DO NOT apply cityID. ✓
│   │                                    #   DO NOT use a city_id IS NULL guard — a migration
│   │                                    #   gap or data-corruption event could leave city_id = NULL
│   │                                    #   for a returning user, causing their region to be silently
│   │                                    #   overwritten on next login. The xmax signal is the only
│   │                                    #   reliable indicator of a fresh insert.
│   ├── refresh_usecase.go               # rotate refresh token, revoke old JTI
│   ├── logout_usecase.go                # add JTI to Redis revocation set
│   ├── delete_account_usecase.go        # 1. extract userID from JWT sub claim
│   │                                    #    (DELETE /user/account has no :userID path param;
│   │                                    #     the JWT sub IS the userID — no additional validation needed)
│   │                                    # 2. soft-delete (deleted_at = NOW())
│   │                                    # 3. revoke all JTIs
│   │                                    # 4. pg NOTIFY account_deleted channel
│   │                                    # 5. schedule hard-delete (30-day grace)
│   │                                    # 6. write audit entry → 204
│   ├── log_consent_usecase.go           # idempotent by userID+type+version
│   │                                    # VALID consentType VALUES — ALLOWLIST ENFORCED:
│   │                                    #   "privacy_policy" | "tos"
│   │                                    # Any other consentType (including "ai_disclaimer")
│   │                                    # MUST be rejected with HTTP 400:
│   │                                    #   { "error": "invalid_consent_type" }
│   │                                    # Guard (run before DB insert):
│   │                                    #   validTypes := map[string]bool{
│   │                                    #     "privacy_policy": true, "tos": true,
│   │                                    #   }
│   │                                    #   if !validTypes[req.ConsentType] {
│   │                                    #     return http.StatusBadRequest, ErrInvalidConsentType
│   │                                    #   }
│   │                                    # The AI Disclaimer is displayed client-side for
│   │                                    # transparency but never logged as a consent event.
│   ├── verify_receipt_usecase.go        # Google Play receipt → INSERT ON CONFLICT DO NOTHING
│   │                                    # status transitions: pending → verified | failed
│   │                                    # illegal transition guard: if row already verified,
│   │                                    #   return existing record (idempotent).
│   ├── restore_subscription_usecase.go  # Google Play purchase restore (Android only)
│   │                                    # Queries Google Play Developer API for active
│   │                                    # subscriptions linked to the user's Google account.
│   │                                    # On active subscription found: update subscription
│   │                                    #   record, issue new JWT with updated tier claim.
│   │                                    # On NO active subscription found:
│   │                                    #   → HTTP 404 { "error": "no_active_subscription" }
│   │                                    #   Client surfaces: "No active subscription found
│   │                                    #   for this account."
│   │                                    # On any other error: HTTP 500 (client shows generic
│   │                                    #   retry error; stays on Paywall screen).
│   │                                    # KILL-SWITCH NOTE (GAP-M1): kill_switch_payments is
│   │                                    #   NOT checked server-side in this usecase. The gate
│   │                                    #   is client-side only (client hides the "Restore" UI
│   │                                    #   when the flag is active). This is an accepted v1
│   │                                    #   trade-off. If server-side gating is needed in future,
│   │                                    #   add: flagRepo.Get("kill_switch_payments") == "true"
│   │                                    #     → HTTP 503 { "error": "payments_unavailable" }
│   │                                    #   before calling the Google Play Developer API.
│   ├── evaluate_thresholds_usecase.go   # debounce 1hr per rule
│   ├── deliver_alert_usecase.go         # FCM push (Android only) + pg NOTIFY alert_delivered channel
│   │                                    # GAP-07 fix: FCM DATA PAYLOAD CONTRACT (canonical source):
│   │                                    #   {
│   │                                    #     "type":      "price_alert",
│   │                                    #     "metal":     "gold" | "silver",
│   │                                    #     "direction": "above" | "below",   ← required field
│   │                                    #     "threshold": "<float as string>",
│   │                                    #     "city_id":   "<cityID>",
│   │                                    #     "screen":    "rates"
│   │                                    #   }
│   │                                    # All six fields are REQUIRED. PRD §9 previously omitted
│   │                                    # "direction" from its example — that was a documentation
│   │                                    # error. §8 AC and MahaSwarnMessagingService.kt both include
│   │                                    # "direction"; this file is now the canonical server-side
│   │                                    # definition. Android client reads all six fields from
│   │                                    # remoteMessage.data[].
│   ├── register_device_token_usecase.go # UPSERT ON CONFLICT (userID, deviceID) DO UPDATE
│   └── set_flag_usecase.go              # SetFlag() → Redis write-through + pg NOTIFY flag_updated
│
├── infrastructure/
│   ├── user_repository.go
│   ├── session_repository.go
│   ├── consent_log_repository.go        # INSERT only; REVOKE UPDATE,DELETE enforced at DB level
│   ├── subscription_repository.go       # UPSERT via INSERT ... ON CONFLICT DO NOTHING
│   ├── receipt_log_repository.go        # append-only, immutable
│   ├── alerts_repository.go
│   ├── device_token_repository.go
│   ├── flags_repository.go
│   ├── audit_log_repository.go          # append-only
│   ├── google_play_client.go            # Google Play Developer API — Android IAP verification
│   │                                    # iOS App Store client not supported; add alongside APPLE_BUNDLE_ID + APPLE_SHARED_SECRET when extending.
│   ├── otp_provider.go                  # OTP provider abstraction + dual-provider dispatcher
│   │                                    #
│   │                                    # OTP AUTH ARCHITECTURE — DUAL PROVIDER:
│   │                                    # Firebase Authentication (primary) + MSG91 SMS (fallback).
│   │                                    # Provider is selected per-request based on OTP_PROVIDER flag.
│   │                                    #
│   │                                    # INTERFACE:
│   │                                    #   type OtpProvider interface {
│   │                                    #     SendOTP(ctx, phone string) error
│   │                                    #     VerifyOTP(ctx, phone, code string) (bool, error)
│   │                                    #   }
│   │                                    #
│   │                                    # FIREBASE FLOW (primary):
│   │                                    #   Send OTP:
│   │                                    #     Handled entirely client-side by Firebase Auth SDK.
│   │                                    #     The Android app calls firebaseAuth.signInWithPhoneNumber()
│   │                                    #     which sends the SMS via Firebase and returns a
│   │                                    #     PhoneAuthCredential with a Firebase ID token on success.
│   │                                    #     Backend SendOTP() is a no-op for Firebase — return nil.
│   │                                    #   Verify OTP:
│   │                                    #     Client sends { phone, firebaseIdToken } to POST /auth/login.
│   │                                    #     Backend verifies firebaseIdToken via Firebase Admin SDK:
│   │                                    #       firebaseToken, err := firebaseAuthClient.VerifyIDToken(ctx, idToken)
│   │                                    #       if err != nil → reject (invalid/expired token)
│   │                                    #       phoneInToken := firebaseToken.Claims["phone_number"].(string)
│   │                                    #       if phoneInToken != normalise(phone) → reject (phone mismatch)
│   │                                    #     On success: issue MahaSwarna JWT pair.
│   │                                    #
│   │                                    # MSG91 FLOW (fallback / standalone):
│   │                                    #   Send OTP:
│   │                                    #     POST https://api.msg91.com/api/v5/otp
│   │                                    #       authkey: MSG91_AUTH_KEY
│   │                                    #       template_id: MSG91_TEMPLATE_ID  (DLT-registered)
│   │                                    #       mobile: 91{phone}  (always prefix country code 91)
│   │                                    #       expiry: MSG91_OTP_EXPIRY_MINUTES
│   │                                    #     Backend generates and stores a hash of the OTP in Redis:
│   │                                    #       key: otp:{phone}  value: bcrypt(otp)  TTL: expiry+60s
│   │                                    #   Verify OTP:
│   │                                    #     Client sends { phone, otp } to POST /auth/login.
│   │                                    #     GET https://api.msg91.com/api/v5/otp/verify
│   │                                    #       authkey: MSG91_AUTH_KEY
│   │                                    #       mobile: 91{phone}
│   │                                    #       otp: <submitted code>
│   │                                    #     On 200 + type=="success" → verified.
│   │                                    #     Rate limit: MSG91 enforces per-number rate limiting;
│   │                                    #       backend additionally enforces: max 5 OTP sends per
│   │                                    #       phone per hour via Redis counter otp_send:{E164phone}.
│   │                                    #       E164 normalisation MUST be applied before constructing
│   │                                    #       the key — using a raw/un-normalised phone string creates
│   │                                    #       separate counters for "+91XXXXXXXXXX" vs "91XXXXXXXXXX"
│   │                                    #       vs "0XXXXXXXXXX", allowing the limit to be bypassed.
│   │                                    #
│   │                                    # DISPATCH LOGIC (OTP_PROVIDER flag):
│   │                                    #   "firebase" → FirebaseOtpProvider only
│   │                                    #   "msg91"    → Msg91OtpProvider only
│   │                                    #   "both"     → FirebaseOtpProvider first; on error/timeout
│   │                                    #                → Msg91OtpProvider (automatic fallback)
│   │                                    #   Login request body must include a provider hint:
│   │                                    #     { phone, otp?, firebaseIdToken?, provider: "firebase"|"msg91" }
│   │                                    #   provider field drives which verification path is taken.
│   │                                    #   If provider is absent, server infers from field presence:
│   │                                    #     firebaseIdToken present → firebase path
│   │                                    #     otp present            → msg91 path
│   │                                    #
│   │                                    # PHONE NORMALISATION (applied before any provider call):
│   │                                    #   All phones stored + compared in E.164 format: +91XXXXXXXXXX
│   │                                    #   Strip leading 0, +91, 91, spaces, hyphens before normalising.
│   │                                    #   Firebase claims["phone_number"] is already E.164.
│   │                                    #   MSG91 mobile field requires "91XXXXXXXXXX" (no +).
│   │                                    #   Normalise(phone string) → "+91" + last10Digits(phone)
│   │                                    #
│   │                                    # RESEND THROTTLE:
│   │                                    #   Redis key: otp_send:{E164phone}  INCR + EXPIRE 3600s
│   │                                    #   On INCR result > 5: return HTTP 429 "too_many_otp_requests"
│   │                                    #   before calling any provider. Applies to both providers.
│   │                                    #
│   │                                    # FIREBASE RATE-LIMIT BOUNDARY (required clarity):
│   │                                    #   FirebaseTooManyRequestsException is a CLIENT-SIDE error
│   │                                    #   surfaced by the Firebase Auth SDK on the Android device.
│   │                                    #   When this fires, the client DOES NOT call POST /auth/login;
│   │                                    #   the error is shown locally ("Too many attempts — try again later")
│   │                                    #   and the MSG91 fallback is NOT triggered.
│   │                                    #   The server-side MSG91 fallback in DualOtpProvider is triggered
│   │                                    #   ONLY by Firebase Admin SDK infrastructure errors (timeout,
│   │                                    #   SDK internal crash) — NOT by credential errors or rate-limit
│   │                                    #   responses from the Firebase Admin SDK.
│   │                                    #   IMPLEMENTATION GUARD (in firebase_otp_provider.go):
│   │                                    #     Firebase Admin SDK VerifyIDToken returns specific error codes
│   │                                    #     for credential failures (auth/id-token-expired,
│   │                                    #     auth/id-token-invalid). These must return HTTP 401 immediately.
│   │                                    #     Only network-level errors (context.DeadlineExceeded, io.EOF,
│   │                                    #     connection refused) should propagate to DualOtpProvider as
│   │                                    #     infrastructure errors triggering the MSG91 fallback.
│   │                                    #   DO NOT add a "retry any Firebase error via MSG91" path —
│   │                                    #   that would bypass Firebase's intentional rate-limit enforcement.
│   │                                    #
│   │                                    # AUDIT:
│   │                                    #   Every OTP send + verify attempt (success or failure) is
│   │                                    #   logged to audit_log: actor=phone, action=otp_send|otp_verify,
│   │                                    #   metadata={provider, success, ip}. Used for abuse detection.
│   ├── firebase_otp_provider.go         # FirebaseOtpProvider: wraps firebase.google.com/go/auth v4
│   │                                    #   SendOTP: no-op (client-side flow)
│   │                                    #   VerifyOTP: VerifyIDToken + phone claim extraction
│   │                                    #   firebaseAuthClient initialised from FIREBASE_SERVICE_ACCOUNT_JSON
│   ├── msg91_otp_provider.go            # Msg91OtpProvider: HTTP client wrapping MSG91 v5 REST API
│   │                                    #   SendOTP:   POST api.msg91.com/api/v5/otp
│   │                                    #   VerifyOTP: GET  api.msg91.com/api/v5/otp/verify
│   │                                    #   2s per-call context deadline; retry once on 5xx
│   │                                    #   Parses MSG91 JSON response: { "type": "success"|"error" }
│   ├── push_notification_client.go      # FCM only (Android) via Firebase Admin SDK (Go)
│   │                                    # sendToDevice(token, title, body, data map)
│   │                                    # iOS/APNs: not supported; extend this client when adding iOS.
│   ├── rate_projection.go               # local Redis read model for alert threshold evaluation
│   └── db.go
│
├── events/
│   ├── notifier.go                      # pg NOTIFY wrapper: Notify(channel, payload)
│   └── listeners.go                     # pg LISTEN: subscription_activated, subscription_expired,
│                                        #   AND user_created (self-listener for subscription provisioning)
│                                        # REQUIRED: RebuildProjectionFromDB() called at startup,
│                                        #   BEFORE the pg LISTEN goroutine starts. Queries:
│                                        #     SELECT user_id, tier FROM subscriptions
│                                        #     WHERE status = 'active' AND expires_at > NOW()
│                                        #   Closes the race window between core passing /health/ready
│                                        #   and pricing/intelligence starting their own listeners.
│                                        #   Any subscription_activated event fired during that window
│                                        #   would otherwise be permanently lost (pg NOTIFY = fire-and-forget).
│                                        #   /health/ready must return 503 until this catch-up completes.
│                                        # REQUIRED: pass RebuildProjectionFromDB as the onReconnect
│                                        #   callback to pgnotify.NewListener. The catch-up query must
│                                        #   re-run after EVERY listener reconnection, not only at startup.
│                                        #   A transient pg connection loss silently drops any NOTIFY
│                                        #   fired during the reconnect window.
│                                        # REQUIRED: user_created self-listener catch-up:
│                                        #   Core listens on user_created to provision free subscriptions.
│                                        #   Any user_created NOTIFY fired during a core restart is
│                                        #   permanently lost — that user gets no subscription row.
│                                        #   Catch-up query (run at startup AND on every pg reconnect):
│                                        #     SELECT id FROM users
│                                        #     WHERE created_at > NOW() - INTERVAL '5 minutes'
│                                        #     AND id NOT IN (SELECT user_id FROM subscriptions)
│                                        #   For each result: insert free subscription row.
│                                        #   5-minute window covers any realistic restart duration.
│
└── jobs/
    ├── alert_threshold_job.go           # cron every minute, evaluate all active alerts
    ├── subscription_expiry_job.go        # daily cron → expire overdue subs → notify
    └── hard_delete_job.go               # daily cron → hard-delete users past 30-day grace period
                                         #   SELECT id FROM users
                                         #     WHERE deleted_at IS NOT NULL
                                         #     AND deleted_at < NOW() - INTERVAL '30 days'
                                         #     AND hard_deleted_at IS NULL
                                         #   For each: delete user rows within the core schema;
                                         #   write audit entry:
                                         #     Append(actor="system", action="hard_delete",
                                         #       entity="user", metadata={userID, deletedAt, hardDeletedAt})
                                         #   Updates hard_deleted_at = NOW() on users row within the
                                         #   same DB transaction as the core-schema row deletions,
                                         #   BEFORE emitting pg NOTIFY account_deleted. This makes the
                                         #   job idempotent (the SELECT WHERE hard_deleted_at IS NULL
                                         #   guard prevents re-processing) and ensures the row is
                                         #   marked complete even if the NOTIFY consumer (intelligence)
                                         #   processes it before this job loop iteration completes.
                                         #   NOTE: pg NOTIFY is fire-and-forget and asynchronous —
                                         #   "before cascade" means before the NOTIFY call, not before
                                         #   the intelligence listener executes. The listener runs
                                         #   independently and its DELETE is idempotent regardless of
                                         #   timing (deleting already-absent rows is a no-op).
                                         #   Cross-service cleanup: FK constraints cannot
                                         #   cascade across pg schemas. intelligence's shops and
                                         #   invoices tables reference user_id but are in a different
                                         #   schema. Cross-service data purge is handled exclusively
                                         #   via the account_deleted pg NOTIFY listener in
                                         #   intelligence/events/listeners.go — see that file for the
                                         #   explicit DELETE FROM shops / invoices WHERE user_id = $1.
                                         #   hard_delete_job.go emits account_deleted after writing
                                         #   hard_deleted_at to trigger this listener.
                                         #   On job failure: Sentry SEV-2 alert; row remains in grace period.
                                         #   cleanup_old_data.sh verifies count of overdue pending deletes
                                         #     and alerts if > 0 rows remain after job run.
```

**JWT Details:**
- Algorithm: **RS256** — private key signs, public key verifies.
- Access TTL: **15 minutes**. Refresh TTL: **30 days** (stored in DB, revocable via JTI).
- Payload: `sub`, `jti`, `tier` (`FREE | PREMIUM | ADMIN`), `region`, `iat`, `exp`.
- **`region` claim:** set at login time by `login_usecase.go` — reads `users.city_id` from the DB and
  embeds it as `region` in the issued JWT. The user supplies their city at registration
  (`POST /auth/register` body includes `cityID`), which is stored in the `users` table. This
  value is embedded in every subsequent JWT so the gateway's `jwt_pre_validator.go` can inject
  `X-Region` without a DB lookup on each request. Changing a user's city requires issuing a
  new JWT (token refresh); the old token retains the old region for up to 15 minutes.

---

## Pricing

> Port `:4002`. Gold/silver rates + WebSocket realtime server.
> The WebSocket server lives here because it is the highest-frequency consumer of rate updates.

```
src/services/pricing/
├── main.go                              # GRACEFUL SHUTDOWN — REQUIRED:
│                                        # WebSocket connections must be drained before the process exits.
│                                        # A hard kill (docker compose up -d pricing) drops all active WS
│                                        # connections without a close frame — clients see an abrupt disconnect
│                                        # and begin exponential backoff (1s → 2s → ... 60s), causing a
│                                        # reconnect storm on every routine deploy.
│                                        #
│                                        # Required implementation in main.go:
│                                        #   srv := &http.Server{Addr: ":4002", Handler: router}
│                                        #
│                                        #   // Start server
│                                        #   go func() {
│                                        #     if err := srv.ListenAndServeTLS(...); err != http.ErrServerClosed {
│                                        #       log.Fatal(err)
│                                        #     }
│                                        #   }()
│                                        #
│                                        #   // Wait for OS signal
│                                        #   quit := make(chan os.Signal, 1)
│                                        #   signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
│                                        #   <-quit
│                                        #
│                                        #   // Phase 1: stop accepting new WS upgrades (15s grace)
│                                        #   ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
│                                        #   defer cancel()
│                                        #   srv.Shutdown(ctx)   // waits for in-flight HTTP upgrades to complete
│                                        #
│                                        #   // Phase 2: send close frame to all live WS connections
│                                        #   connectionRegistry.CloseAll(websocket.CloseGoingAway, "server restarting")
│                                        #   // connectionRegistry.CloseAll iterates sync.Map, calls
│                                        #   // conn.WriteMessage(websocket.CloseMessage,
│                                        #   //   websocket.FormatCloseMessage(websocket.CloseGoingAway, reason))
│                                        #   // on each connection. Clients receive a proper close frame
│                                        #   // and start reconnect with backoff — no storm behaviour.
│                                        #
│                                        # docker-compose.prod.yml: set stop_grace_period: 20s for pricing
│                                        # to give the 15s HTTP drain + close-frame write time to complete.
│                                        #
│                                        # REQUIRED: connection_registry.go must expose:
│                                        #   func (r *ConnectionRegistry) CloseAll(code int, reason string)
│                                        # Implementation: range over sync.Map, call WriteMessage on each conn.
│
├── http/
│   ├── router.go
│   ├── rates_handler.go                 # GET /rates/:cityID
│   │                                    # GET /rates/:cityID/history
│   │                                    # response: { Gold, Silver, Source: gemini,
│   │                                    #   GeneratedAt, Stale bool }
│   │                                    # source: Gemini AI snapshot (Redis TTL 1h → DB fallback)
│   │                                    #   stale:true if no fresh snapshot within IST window
│   └── middleware/
│       ├── service_auth.go              # validates X-Service-Token HMAC-SHA256 header
│       └── rate_limiter.go
│
├── ws/
│   ├── ws_server.go                     # gorilla/websocket HTTP → WS upgrade
│   │                                    # parses JSON envelope { Channel, Payload }
│   │                                    # routes: rates | alerts
│   │                                    # WS HANDSHAKE RATE LIMIT (gateway middleware
│   │                                    #   is bypassed on :4002; this service must self-enforce):
│   │                                    #   Redis key: ws_hs:{ip} — INCR with 60s EXPIRE
│   │                                    #   Limit: 20 new WS upgrade attempts per IP per minute.
│   │                                    #   Exceeding → HTTP 429 before gorilla upgrade.
│   │                                    #   Also enforce http.MaxBytesReader(w, r.Body, 64*1024)
│   │                                    #   on the initial HTTP upgrade request body.
│   ├── channel_router.go
│   ├── connection_registry.go           # userID → []WebSocket; sync.Map
│   │                                    # Public methods:
│   │                                    #   Register(userID, conn)
│   │                                    #   Remove(userID, conn)
│   │                                    #   Send(userID, msg []byte)
│   │                                    #   CloseAll(code int, reason string)
│   │                                    #     iterates sync.Map, calls WriteMessage(CloseMessage,
│   │                                    #     FormatCloseMessage(code, reason)) on every conn.
│   │                                    #     Called by main.go graceful shutdown (Phase 2).
│   │                                    #     Must be exported and defined here — not in main.go.
│   │                                    # sync.Map is adequate to ~5k concurrent connections.
│   │                                    # At >5k: replace with ShardedMap (16 shards,
│   │                                    #   CRC32(userID) % 16 as shard selector).
│   │                                    # Upgrade trigger: WS concurrent connections > 5,000
│   ├── heartbeat.go                     # ping/pong keepalive; dead connection pruning
│   │                                    # HEARTBEAT SPEC (required to bound stale
│   │                                    #   sync.Map entries from network-drop disconnects):
│   │                                    #   Ping interval:          30 seconds
│   │                                    #   Pong read deadline:     10 seconds after ping sent
│   │                                    #   Max stale entry window: 40 seconds (30s interval
│   │                                    #     + 10s pong deadline) before connection is pruned.
│   │                                    #   Implementation:
│   │                                    #     conn.SetReadDeadline(time.Now().Add(40 * time.Second))
│   │                                    #     conn.SetPongHandler(func(string) error {
│   │                                    #       conn.SetReadDeadline(time.Now().Add(40 * time.Second))
│   │                                    #       return nil
│   │                                    #     })
│   │                                    #     ticker := time.NewTicker(30 * time.Second)
│   │                                    #     // send ping on each tick; gorilla closes conn on
│   │                                    #     // read deadline exceeded → registry.Remove(userID)
│   │                                    #   On abnormal disconnect (no close frame), the read
│   │                                    #   deadline fires within 40s and the gorilla websocket
│   │                                    #   read loop returns an error, triggering registry cleanup.
│   ├── ban_service.go                   # force-disconnect on UserBanned event
│   └── redis_fanout.go                  # BufferedFanout: 50ms batch window
│                                        # Latest-only fanout: within each 50ms window, only
│                                        #   the most recent rate per cityID is pushed.
│                                        #   Intermediate updates are intentionally dropped —
│                                        #   gold rates are sampled observations, not a ledger.
│                                        #   If every tick is needed, remove the batch window.
│
├── domain/
│   ├── gold_rate.go
│   ├── city.go
│   └── ai_rate_snapshot.go
│
├── application/
│   ├── get_rate_usecase.go              # reads Redis snapshot; stale:true outside IST window
│   │                                    # COLD-START EDGE CASE (empty DB + Gemini down):
│   │                                    #   If no snapshot exists in Redis OR DB for a city AND
│   │                                    #   Gemini is unreachable:
│   │                                    #     → return HTTP 404 with body:
│   │                                    #         { "error": "city_rates_not_available",
│   │                                    #           "message": "Rates not yet available for this city" }
│   │                                    #   Do NOT return a zero rate or a 503 (which implies retry).
│   │                                    #   404 signals the client to show a "Rates not available yet —
│   │                                    #   check back shortly" state, not a retry spinner.
│   │                                    #   This only occurs on first-ever startup before Gemini
│   │                                    #   has run its first scheduled fetch. Normal operation
│   │                                    #   is unaffected once any snapshot exists.
│   ├── get_history_usecase.go
│   └── generate_ai_rates_usecase.go     # city list → Gemini API (via gemini_client.go)
│                                        # batched, concurrency-limited (errgroup, sem=5)
│                                        #
│                                        # GEMINI FAILURE HANDLING (sole AI source — requires explicit
│                                        #   degradation policy):
│                                        #   on Gemini API error / timeout (2s per-city deadline):
│                                        #     → last known snapshot preserved as-is
│                                        #     → serve existing snapshot with stale:true
│                                        #     → pg NOTIFY rate_stale (Alertmanager + SEV-2)
│                                        #     → log to Sentry with error detail
│                                        #   on 3 consecutive full-run failures (all cities fail):
│                                        #     → escalate to SEV-1 via PagerDuty
│                                        #     → activate kill_switch_ws feature flag via direct DB,
│                                        #       BUT ONLY AFTER raising the BFF rate limit first.
│                                        #       REQUIRED ORDER (OQ-8 gate — automated, not manual):
│                                        #         STEP 1: raise rate limit BEFORE flipping kill-switch:
│                                        #           UPDATE feature_flags SET value = '60'
│                                        #           WHERE key = 'rate_limit_bff_free_rpm';
│                                        #         STEP 2: wait 5 seconds for Redis flag cache to refresh.
│                                        #         STEP 3: flip WS kill-switch:
│                                        #           UPDATE feature_flags SET value = 'true'
│                                        #           WHERE key = 'kill_switch_ws';
│                                        #       This sequence MUST be encoded as a single runbook script
│                                        #       (scripts/activate_ws_killswitch.sh) for manual ops use,
│                                        #       and replicated in generate_ai_rates_usecase.go's
│                                        #       3-failure escalation path using set_flag_usecase.go
│                                        #       calls in the correct order with a 5s sleep between:
│                                        #         flagUsecase.SetFlag(ctx, "rate_limit_bff_free_rpm", "60")
│                                        #         time.Sleep(5 * time.Second)
│                                        #         flagUsecase.SetFlag(ctx, "kill_switch_ws", "true")
│                                        #       sends ~40 RPS of unspread polling against a gateway still
│                                        #       enforcing the 40 RPM FREE-tier limit — 429 storm.
│                                        #       The script asserts rate_limit_bff_free_rpm is updated
│                                        #       before setting kill_switch_ws, then sleeps 5s.
│                                        #       Also verify client ±5s jitter is deployed (check APK
│                                        #       version on Play Console) before activating at full DAU.
│                                        #       client degrades to polling REST (30s ±5s jitter interval)
│                                        #       which serves stale:true banners — user is visibly informed
│                                        #   MANUAL OVERRIDE PATH (admin UI or direct DB — documented here
│                                        #   as the emergency procedure when Gemini is down for >1 IST session):
│                                        #     INSERT INTO ai_rate_snapshots
│                                        #       (city_id, gold_rate, silver_rate, source, generated_at)
│                                        #     VALUES ($city, $gold, $silver, 'manual_override', NOW())
│                                        #     ON CONFLICT (city_id) DO UPDATE
│                                        #       SET gold_rate=EXCLUDED.gold_rate,
│                                        #           silver_rate=EXCLUDED.silver_rate,
│                                        #           source=EXCLUDED.source,
│                                        #           generated_at=EXCLUDED.generated_at;
│                                        #     Then: bash scripts/warmup_cache.sh (re-warms Redis)
│                                        #     RateSource on invoices will read "manual_override" — client
│                                        #     shows a warning banner. Runbook: docs/runbooks/gemini_outage.md
│                                        #   NOTE: A MCX API integration (MCX gold spot) as a secondary
│                                        #   automated source is the long-term fix for Gemini sole-source risk.
│                                        #   Add as gemini_client fallback when Gemini fails ≥2 consecutive runs.
│                                        #   Scope: post-launch, tracked in the backlog.
│                                        #
│                                        # on success: SANITY CHECK before writing snapshot:
│                                        #   if prevSnapshot exists AND
│                                        #   abs(newRate - prevRate) / prevRate > 0.02 (2%):
│                                        #     → do NOT write new snapshot
│                                        #     → serve prevSnapshot with stale:true
│                                        #     → pg NOTIFY rate_stale (Alertmanager + SEV-2)
│                                        #     → log anomaly to Sentry with both values
│                                        #   A confidently-wrong fresh rate is worse than a
│                                        #   stale banner. Threshold 2% (not 5%) — gold intraday
│                                        #   moves are typically 0.3–1.0%; a 5% gate accepts
│                                        #   ₹300/gram errors on a ₹6000/gram price silently.
│                                        #   2% still passes normal market volatility and rejects
│                                        #   obvious Gemini hallucinations. Configurable via
│                                        #   feature flag "rate_sanity_threshold_pct".
│                                        # on sanity pass: writes snapshot → DB + Redis
│                                        # pg NOTIFY ai_rate_snapshot_ready per city
│
├── infrastructure/
│   ├── rates_repository.go              # write: pgx; read: Redis
│   ├── ai_rate_snapshot_repository.go   # InsertSnapshot, GetLatest, GetHistory, Prune
│   ├── redis_cache.go                   # rates:latest:ai:{cityID} TTL 3600s
│   ├── gemini_client.go                 # google/generative-ai-go SDK wrapper
│   │                                    # GenerateRates(cityID) → GoldRate, SilverRate
│   └── db.go
│
├── watchdog/
│   └── rate_quality_watchdog.go         # single goroutine; per-city scan runs two quality
│                                        # checks in sequence:
│                                        #   1. STALENESS: if no fresh snapshot within IST window
│                                        #        → serve prevSnapshot with stale:true
│                                        #        → pg NOTIFY rate_stale (Alertmanager + SEV-2)
│                                        #   2. SANITY: if abs(new - prev) / prev > threshold (2%)
│                                        #        → reject new snapshot; keep prev with stale:true
│                                        #        → pg NOTIFY rate_stale; log anomaly to Sentry
│                                        #   threshold read from feature flag "rate_sanity_threshold_pct"
│                                        #   default: 2% — tighter than 5% because gold intraday moves
│                                        #   are 0.3–1.0%; a 5% gate silently accepts ₹300/gram errors.
│                                        #   Configurable without deploy via feature flag.
│                                        #   no prev snapshot → pass (first run)
│
├── events/
│   ├── notifier.go                      # pg NOTIFY: rate_updated, rate_stale, ai_rate_snapshot_ready
│   └── listeners.go                     # pg LISTEN: user_banned → ban_service.Disconnect()
│                                        # REQUIRED: RebuildSubscriptionProjectionViaAPI() called at
│                                        #   startup before LISTEN goroutine. Same pattern as core/events.
│                                        #   /health/ready returns 503 until catch-up completes.
│                                        # REQUIRED: pass RebuildSubscriptionProjectionViaAPI as
│                                        #   onReconnect callback to pgnotify.NewListener so the
│                                        #   catch-up re-runs after every reconnection.
│                                        # RETRY POLICY FOR STARTUP API CALL:
│                                        #   core may still be starting when pricing calls
│                                        #   GET http://core:4001/internal/subscriptions/active.
│                                        #   Retry with exponential backoff before failing /health/ready:
│                                        #     attempts: 8
│                                        #     delays:   1s → 2s → 4s → 8s → 16s → 32s → 64s → 128s
│                                        #     total max wait: ~255s (~4.25 min)
│                                        #     on all attempts exhausted: log SEV-2 to Sentry;
│                                        #       /health/ready returns 503 (prevents gateway routing
│                                        #       traffic to a pricing node with stale projection).
│                                        #   RATIONALE: core may be running its own startup catch-up
│                                        #   queries (20–40s on a cold DB). 8 attempts gives ~4.25 min
│                                        #   of grace. docker-compose.prod.yml MUST set
│                                        #   start_period: 300s for pricing + intelligence services.
│                                        #   Same retry policy applies on every pg reconnect callback.
│                                        # CROSS-SCHEMA NOTE: pricing must NOT directly query
│                                        #   core's subscriptions table — cross-schema reads violate
│                                        #   the service schema isolation invariant.
│                                        #   RebuildSubscriptionProjectionViaAPI() calls:
│                                        #     GET http://core:4001/internal/subscriptions/active
│                                        #     (X-Service-Token header required)
│                                        #   core exposes this endpoint for pricing + intelligence
│                                        #   startup rebuild only. Returns [{userID, tier}] list.
│
└── jobs/
    └── ai_rate_scheduler_job.go         # cron: 0 10-19 * * 1-6
                                         # CRITICAL — TIMEZONE: robfig/cron defaults to UTC.
                                         #   The cron runner MUST be initialised with IST explicitly:
                                         #     istLoc, _ := time.LoadLocation("Asia/Kolkata")
                                         #     c := cron.New(cron.WithLocation(istLoc))
                                         #   Without this, "0 10-19 * * 1-6" fires at 10:00–19:00 UTC
                                         #   = 15:30–00:30 IST — entirely wrong market window.
                                         #   Do NOT rely on the IST hour guard as the primary gate;
                                         #   it is defense-in-depth only against cron timezone drift,
                                         #   not a substitute for correct cron initialisation.
                                         # Runs at :00 of every hour from 10:00–19:00 IST, Mon–Sat.
                                         # IST hour guard (defense-in-depth, not primary gate):
                                         #   hour < 10 || >= 20 → skip
                                         # sync.Mutex guard (not distributed lock — single VPS,
                                         #   single cron instance; add Redis lock only at ~50k DAU
                                         #   when a second pricing node is introduced)
                                         # PagerDuty SEV-2 on 3 consecutive failures
```

---

## Intelligence

> Port `:4003`. Jewelry design catalog and marketplace.
> Gemini Vision is used for banner content moderation only.

```
src/services/intelligence/
├── main.go
│
├── http/
│   ├── router.go
│   ├── catalog_handler.go               # GET /catalog/search?q=&region=&page=&limit=
│   │                                    # GET /catalog/recommend?region=&page=&limit=
│   │                                    # GET /catalog/designs/:id  ← design detail endpoint
│   │                                    #   VIEW COUNT INCREMENT — REQUIRED:
│   │                                    #   When this endpoint is called (design detail view),
│   │                                    #   the handler MUST call:
│   │                                    #     viewCountCache.IncrViewCount(designID)
│   │                                    #   The Android client does NOT make a separate increment
│   │                                    #   request (PRD §4.3 AC). Server-side increment on the
│   │                                    #   detail fetch is the only increment point.
│   │                                    #   IncrViewCount uses an atomic Lua INCR+EXPIRE in
│   │                                    #   view_count_cache.go (see that file for implementation).
│   │                                    #   flush_view_counts_job.go flushes to DB every 5 min.
│   │                                    #   NOTE: GET /catalog/search and GET /catalog/recommend
│   │                                    #   do NOT call IncrViewCount — only the detail fetch does.
│   │                                    #   Calling it on list endpoints would increment every design
│   │                                    #   that appears in any result set, inflating counts.
│   │                                    #
│   │                                    # ⚠️  POST /catalog/image-search — DO NOT IMPLEMENT YET.
│   │                                    #   This route is intentionally absent from router.go.
│   │                                    #   The Android client has killSwitchImageSearch=true by
│   │                                    #   default, so the frontend will never call this endpoint.
│   │                                    #   Adding a stub handler before the feature is fully
│   │                                    #   designed creates a half-working route that passes CI
│   │                                    #   but cannot be safely used in production (no Gemini
│   │                                    #   Vision image-embedding pipeline, no similarity search,
│   │                                    #   no result ranking, no quota controls).
│   │                                    #   Implementation gate: BOTH of the following must be true
│   │                                    #   before this route is added:
│   │                                    #     1. Full backend handler is implemented and tested
│   │                                    #        (Gemini Vision embedding + design_catalog similarity)
│   │                                    #     2. Android killSwitchImageSearch set to false in the
│   │                                    #        same release (backend + frontend ship together).
│   │                                    #   Tracked in backlog as: CATALOG-IMAGE-SEARCH-v1.
│   │                                    #   If a route stub is needed for local dev testing only,
│   │                                    #   add it behind an APP_ENV=development guard that returns
│   │                                    #   HTTP 501 Not Implemented in staging and production.
│   │                                    #
│   │                                    # On any handler that invokes Gemini (AI recommendations),
│   │                                    # after the Gemini SDK call completes,
│   │                                    # extract usage metadata and write internal quota headers:
│   │                                    #   w.Header().Set("X-Internal-Ai-Quota-Used",     strconv.Itoa(used))
│   │                                    #   w.Header().Set("X-Internal-Ai-Quota-Limit",    strconv.Itoa(limit))
│   │                                    #   w.Header().Set("X-Internal-Ai-Quota-Reset-At", strconv.FormatInt(resetAt, 10))
│   │                                    # Gateway's ai_quota_interceptor.go reads these and
│   │                                    # re-emits as X-Ai-Quota-* to the Android client.
│   │                                    # Non-Gemini handlers: omit headers (client keeps last-known values).
│   ├── shop_handler.go                  # POST /shops, GET /shops
│   │                                    # POST /shops/:id/banner → { uploadURL, objectKey }
│   │                                    # POST /shops/:id/banner/confirm → moderation→resize
│   ├── invoice_handler.go               # POST /shops/:id/invoice/generate
│   │                                    #   auth: JWT (shopkeeper must own shop_id)
│   │                                    #   body: GenerateInvoiceRequest
│   │                                    #   → generate_invoice_usecase → PDF bytes (NOT S3)
│   │                                    #   response: InvoiceResponse { invoiceID, pdfBytes []byte,
│   │                                    #     generatedAt, rateSource }  — ADR-001 DECIDED.
│   │                                    #   PDF is NOT stored server-side; raw bytes returned
│   │                                    #   directly. No CDN URL, no S3 key.
│   │                                    #   rate limit: 60 invoices/shop/day (Redis counter)
│   │                                    #   Redis key: invoice_count:{shopID}:{YYYY-MM-DD-IST}
│   │                                    #     where YYYY-MM-DD-IST is today's date in Asia/Kolkata.
│   │                                    #     Key is per-shop (NOT per-user): multiple authenticated
│   │                                    #     owners of the same shop share the same counter.
│   │                                    #     shopID is taken from the URL path param — NEVER from
│   │                                    #     the JWT sub claim (which is the userID).
│   │                                    #   TTL: set to seconds-until-midnight-IST on key creation.
│   │                                    #     Compute: time.Until(nextMidnightIST(time.Now()))
│   │                                    #     Key expires naturally at the IST day boundary.
│   │                                    #   On limit exceeded: HTTP 429 with body:
│   │                                    #     { "error": "invoice_daily_limit_exceeded" }
│   │                                    #   Client ApiErrorMapper discriminates this body
│   │                                    #   from a generic 429 (RateLimited) and maps to
│   │                                    #   ApiError.InvoiceLimitExceeded with message:
│   │                                    #   "Invoice limit reached for today — try again tomorrow."
│   │                                    #   Integration test REQUIRED: seed two users on the same
│   │                                    #   shop; assert their combined daily total is capped at 60.
│   └── middleware/
│       ├── service_auth.go              # validates X-Service-Token HMAC-SHA256 header
│       ├── jwt_auth.go
│       └── rate_limiter.go
│
├── domain/
│   ├── design.go                        # ID, Title, Description, Category, Style,
│   │                                    #   Region, MetalType, ImageURL, Tags, CreatedAt
│   ├── shop.go
│   └── invoice.go                       # InvoiceID, ShopID, UserID, CustomerName,
│                                        #   CustomerPhone, Items []InvoiceLineItem,
│                                        #   Subtotal, Total, PaymentMode, Notes,
│                                        #   PdfSizeBytes int, GeneratedAt
│                                        #   NOTE: No PdfObjectKey — PDF is not stored server-side
│                                        #   (ADR-001). PdfSizeBytes is recorded for audit only.
│                                        #   InvoiceLineItem: Description, MetalType,
│                                        #     WeightGrams, RatePerGram, MakingCharges, Amount
│
├── application/
│   ├── search_design_usecase.go         # PostgreSQL tsvector full-text search
│   │                                    # region match → rank +0.3; trending → rank +0.1
│   │                                    # empty result → trending fallback (ORDER BY view_count)
│   ├── recommend_design_usecase.go      # Redis cache: catalog:rec:{region}:{page} TTL 300s
│   │                                    # query: SELECT * FROM design_catalog
│   │                                    #   WHERE region = $1 OR region IS NULL
│   │                                    #   ORDER BY view_count DESC
│   │                                    #   LIMIT $2 OFFSET $3
│   │                                    # view_count incremented via Redis INCR
│   │                                    #   design:views:{designID} on each view event
│   │                                    #   flushed to DB by flush_view_counts_job.go (every 5 min)
│   ├── register_shop_usecase.go         # gated by subscription tier
│   ├── get_banner_upload_url_usecase.go # validates shop ownership; presigned S3 URL
│   ├── confirm_banner_upload_usecase.go # Gemini Vision moderation → image resize → store
│   │                                    # STEP 1 OF TWO-STEP RESIZE PIPELINE:
│   │                                    #   After moderation passes, resize the uploaded
│   │                                    #   banner to a maximum of 1200×400 px before
│   │                                    #   writing to CDN storage. This enforces the
│   │                                    #   PDF invoice size budget and is step (a).
│   │                                    # STEP 2 is in invoice_pdf_builder.go (max 600×160
│   │                                    #   px, JPEG quality 80, before PDF embedding).
│   │                                    # Integration tests validating banner dimensions must
│   │                                    #   account for both steps: CDN-stored image ≤1200×400;
│   │                                    #   in-PDF image ≤600×160.
│   └── generate_invoice_usecase.go      # 1. validate shop ownership (shopID must belong to JWT sub)
│                                        # 2. fetch shop record → name, address, GST, phone, bannerCDNURL
│                                        # 3. resolve gold/silver rate for shop's city:
│                                        #      a. if GenerateInvoiceRequest.GoldRateOverride > 0:
│                                        #           use client-supplied rate (client has live WS rate)
│                                        #           set RateSource = "client_override"
│                                        #      b. else: read from Redis snapshot (rates_repository.go)
│                                        #           if Redis miss or circuit breaker open:
│                                        #             → return 503 with error "rate_unavailable"
│                                        #             → do NOT silently embed a zero or stale rate
│                                        #           RATE SOURCE RESOLUTION (in priority order):
│                                        #             snapshot.Source == "manual_override"
│                                        #               → RateSource = "manual_override"
│                                        #               (admin manually inserted rate during Gemini outage)
│                                        #             snapshot.Stale == true
│                                        #               → RateSource = "stale"
│                                        #             else
│                                        #               → RateSource = "live"
│                                        #           This ensures client receives the correct warning:
│                                        #             "manual_override" → "Invoice uses a manually set rate"
│                                        #             "stale"           → "Invoice uses a stale rate"
│                                        #             "live"            → no warning shown
│                                        # 4. compute line item totals + grand total
│                                        # 5. build InvoicePDFData { shop, customer, items, totals,
│                                        #      bannerURL, invoiceNumber, generatedAt (IST), rateSource }
│                                        # 6. invoice_pdf_builder.BuildPDF(data) → []byte
│                                        # 7. insert invoice record (invoice_repository) with
│                                        #      pdf_size_bytes (no S3 key — PDF is not stored server-side)
│                                        # 8. return InvoiceResponse { invoiceID, pdfBytes []byte,
│                                        #       generatedAt, rateSource }
│                                        #    GAP-04 fix: rateSource is one of FOUR values:
│                                        #      "client_override" — GoldRateOverride > 0 (step 3a)
│                                        #      "manual_override" — admin-inserted snapshot (step 3b)
│                                        #      "stale"           — snapshot.Stale == true (step 3b)
│                                        #      "live"            — fresh Gemini snapshot (step 3b)
│                                        #    Previously documented as three values — "manual_override"
│                                        #    was omitted. All four values are handled in step 3 and
│                                        #    must be covered by generate_invoice_usecase_test.go.
│                                        #    PDF bytes are streamed directly to the client.
│                                        #    Client is responsible for persisting to local device storage.
│
├── infrastructure/
│   ├── design_repository.go             # GetByID, FullTextSearch, GetByRegion
│   ├── shop_repository.go
│   ├── invoice_repository.go            # InsertInvoice: stores invoiceID, shopID, userID,
│   │                                    #   customerName, itemsJson, totals, paymentMode,
│   │                                    #   pdf_size_bytes, generatedAt.
│   │                                    #   No pdf_object_key — PDF is not stored server-side.
│   │                                    #   Invoice history retrieval ships when history screen
│   │                                    #   Add when history screen is built.
│   ├── invoice_pdf_builder.go           # uses signintech/gopdf (pure Go, no CGO)
│   │                                    # layout per page:
│   │                                    #   HEADER: shop banner image (fetched from S3/CDN URL,
│   │                                    #     cached in-process 1h)
│   │                                    #   INR FORMATTING (GAP-4 fix):
│   │                                    #     All monetary values in the PDF (line item totals,
│   │                                    #     grand total, per-gram rate) must use Indian
│   │                                    #     lakh/crore grouping: e.g. ₹61,23,456.00.
│   │                                    #     Use golang.org/x/text/message with language.Hindi
│   │                                    #     or format manually: groups of 2 after the first 3
│   │                                    #     digits from the right.
│   │                                    #     Do NOT use strconv.FormatFloat with plain US grouping.
│   │                                    #   STEP 2 OF TWO-STEP RESIZE PIPELINE (GAP-1 fix):
│   │                                    #     Resize to MAX 600×160 px (aspect-ratio preserving).
│   │                                    #     Re-encode as JPEG at quality 80.
│   │                                    #     Do NOT embed raw PNG or WebP from CDN directly —
│   │                                    #     uncompressed banners can exceed the 500 KB PDF budget.
│   │                                    #     Step 1 (1200×400 px CDN resize) is in
│   │                                    #     confirm_banner_upload_usecase.go; both steps are
│   │                                    #     required for the PDF size NFR (< 500 KB).
│   │                                    #   BANNER FETCH FAILURE FALLBACK:
│   │                                    #     If CDN fetch fails or times out (2s deadline):
│   │                                    #       - Log error to Sentry (non-fatal)
│   │                                    #       - Render shop name as text header instead of image
│   │                                    #       - Do NOT fail the entire PDF generation
│   │                                    #     Cache the banner bytes in a sync.Map keyed by objectKey;
│   │                                    #     TTL: 1h. On cache miss, fetch with 2s timeout.
│   │                                    #     On shop registration (confirm_banner_upload_usecase.go):
│   │                                    #       warm the cache immediately after moderation passes.
│   │                                    #   SHOP INFO: name, address, GST number, phone
│   │                                    #   DIVIDER LINE
│   │                                    #   INVOICE META: Invoice No., Date (IST), Payment Mode
│   │                                    #   CUSTOMER: name, phone (if provided)
│   │                                    #   LINE ITEMS TABLE:
│   │                                    #     cols: Description | Metal | Weight(g) |
│   │                                    #           Rate/g | Making | Amount (₹)
│   │                                    #   TOTALS: Subtotal, Making Charges, Grand Total (₹)
│   │                                    #   FOOTER: "Thank you for your purchase"
│   │                                    #     + "Powered by MahaSwarna" (small, right-aligned)
│   │                                    #   Font: Noto Sans (supports Devanagari for Hindi names)
│   │                                    #   Paper: A4 portrait
│   │                                    #   BuildPDF(data InvoicePDFData) ([]byte, error)
│   ├── view_count_cache.go              # IncrViewCount(designID) → Redis INCR design:views:{designID}
│   │                                    # GetViewCounts(designIDs) → map[string]int64
│   │                                    # used by flush_view_counts_job.go to batch-write to DB
│   │                                    # ATOMICITY REQUIREMENT: INCR + EXPIRE must be atomic.
│   │                                    # A bare INCR followed by a separate EXPIRE call is a race:
│   │                                    # a second goroutine can INCR between the two calls and
│   │                                    # reset the TTL, causing the key to slide indefinitely on
│   │                                    # a popular design. Use a Lua script for atomicity:
│   │                                    #   var incrWithTTL = redis.NewScript(`
│   │                                    #     local v = redis.call("INCR", KEYS[1])
│   │                                    #     if v == 1 then
│   │                                    #       redis.call("EXPIRE", KEYS[1], ARGV[1])
│   │                                    #     end
│   │                                    #     return v
│   │                                    #   `)
│   │                                    #   incrWithTTL.Run(ctx, rdb,
│   │                                    #     []string{"design:views:" + designID}, 600)
│   │                                    # The EXPIRE is set only on the first INCR (v == 1),
│   │                                    # not on every call — TTL is established once per flush
│   │                                    # cycle, not continuously reset by every page view.
│   ├── s3_client.go                     # presigned S3 URL — used for shop banner upload only
│   │                                    # (invoice PDFs are NOT stored in S3; returned as bytes)
│   ├── moderation_client.go             # Gemini Vision content moderation (banner upload only)
│   ├── cdn_url_builder.go               # CDN_BASE_URL + objectKey → signed CDN URL
│   │                                    # used for shop banner serving only; not used for invoices
│   ├── subscription_projection.go       # isPremium(userID) — Redis read model
│   │                                    # Rebuilt via internal API call to core —
│   │                                    # NOT by direct DB query into core's subscriptions table.
│   └── db.go
│
├── jobs/
│   └── flush_view_counts_job.go         # cron every 5 minutes
│                                        # reads design:views:{designID} counters from Redis
│                                        # batch-UPDATE design_catalog SET view_count = view_count + delta
│                                        # resets Redis counters after flush
│                                        # VIEW COUNT KEY TTL: set atomically on first INCR
│                                        #   via Lua script in view_count_cache.go (EXPIRE 600s).
│                                        #   TTL is established once when the key is created
│                                        #   (v == 1), not reset on every page view — this
│                                        #   bounds key lifetime to 10 minutes on job failure.
│                                        #   Accumulated integers expire rather than grow forever.
│                                        #   On flush failure: Sentry SEV-3 log; counters
│                                        #   accumulate safely for up to 10 min then expire.
│
└── events/
    ├── notifier.go                      # pg NOTIFY for shop and invoice events
    └── listeners.go                     # pg LISTEN: subscription_activated, subscription_expired
                                         #   → update subscription_projection
                                         #   account_deleted → delete user catalog/shop data
                                         # CROSS-SCHEMA CASCADE: FK constraints cannot
                                         #   span pg schemas. intelligence must explicitly handle
                                         #   account_deleted events to purge its own tables:
                                         #     DELETE FROM shops WHERE user_id = $userID
                                         #     DELETE FROM invoices WHERE user_id = $userID
                                         #   These deletes run inside the account_deleted listener
                                         #   handler. Test coverage: delete_account_usecase_test.go
                                         #   must assert that shops + invoices rows are absent after
                                         #   the cascade (integration test via testcontainers-go).
                                         # DROPPED NOTIFY COMPENSATING MECHANISM (G-20):
                                         #   pg NOTIFY is fire-and-forget. If the intelligence service
                                         #   is restarting during a user's account deletion, the
                                         #   account_deleted NOTIFY fires and is silently lost —
                                         #   that user's shops and invoices will never be purged
                                         #   (GDPR/compliance violation).
                                         #   REQUIRED: at startup (and on every pg reconnect callback),
                                         #   intelligence must run a catch-up purge query against
                                         #   the core schema via the internal API:
                                         #     Query core: GET /internal/pending-purges
                                         #     (or: check core.users WHERE hard_deleted_at IS NOT NULL
                                         #      AND user_id still present in intelligence.shops or invoices)
                                         #   Simplest implementation (Option A — recommended):
                                         #     At startup, run:
                                         #       DELETE FROM shops WHERE user_id IN (
                                         #         SELECT user_id FROM pending_intelligence_purges)
                                         #       DELETE FROM invoices WHERE user_id IN (
                                         #         SELECT user_id FROM pending_intelligence_purges)
                                         #   where pending_intelligence_purges is a table in the
                                         #   intelligence schema populated by account_deleted listener
                                         #   BEFORE the DELETE — this makes the delete idempotent and
                                         #   recoverable on restart.
                                         #   Alternatively (Option B): hard_delete_job.go in core
                                         #   queries intelligence DB directly as a fallback cleanup step
                                         #   after emitting the NOTIFY (cross-schema only for cleanup).
                                         #   Option A is preferred — keeps intelligence self-contained.
                                         # REQUIRED: RebuildSubscriptionProjectionViaAPI() called at
                                         #   startup before LISTEN goroutine. Calls:
                                         #     GET http://core:4001/internal/subscriptions/active
                                         #     (X-Service-Token header required)
                                         #   RETRY POLICY FOR STARTUP API CALL:
                                         #     core may still be starting when intelligence calls this.
                                         #     Retry with exponential backoff before failing /health/ready:
                                         #       attempts: 8
                                         #       delays:   1s → 2s → 4s → 8s → 16s → 32s → 64s → 128s
                                         #       total max wait: ~255s (~4.25 min)
                                         #       on all attempts exhausted: log SEV-2 to Sentry;
                                         #         /health/ready returns 503.
                                         #     docker-compose.prod.yml MUST set start_period: 300s
                                         #     for intelligence (same as pricing).
                                         #     Same retry policy applies on every pg reconnect callback.
                                         #   CROSS-SCHEMA NOTE: Do NOT directly query
                                         #   core's subscriptions table — cross-schema reads violate
                                         #   the service schema isolation invariant.
                                         #   Prevents PREMIUM users from seeing FREE-tier content
                                         #   after any service restart during active market hours.
                                         #   /health/ready returns 503 until catch-up completes.
                                         # REQUIRED: pass RebuildSubscriptionProjectionViaAPI as
                                         #   onReconnect callback to pgnotify.NewListener so the
                                         #   catch-up re-runs after every reconnection.
```

---

## Infrastructure (shared drivers)

```
src/infrastructure/
│
├── pgnotify/
│   ├── notifier.go                      # pg NOTIFY wrapper: Notify(conn, channel, payload)
│   └── listener.go                      # pg LISTEN: NewListener(dsn, channel, onReconnect func) → chan Event
│                                        # reconnects on connection loss with exponential backoff
│                                        # CRITICAL: pg NOTIFY is fire-and-forget. Any NOTIFY fired
│                                        #   while the listener is mid-reconnect is permanently lost.
│                                        #   NewListener accepts an onReconnect callback that callers
│                                        #   MUST use to re-run their catch-up query after every
│                                        #   reconnection — not only at startup.
│                                        #   core/events/listeners.go:    onReconnect = RebuildProjectionFromDB
│                                        #   pricing/events/listeners.go: onReconnect = RebuildSubscriptionProjectionViaAPI
│                                        #   intelligence/events/listeners.go: onReconnect = RebuildSubscriptionProjectionViaAPI
│                                        #   (pricing + intelligence call the internal core API,
│                                        #    not the DB directly, to avoid cross-schema reads)
│                                        #   Without this, a transient pg hiccup during a
│                                        #   subscription_activated event silently leaves a user
│                                        #   on FREE tier until the next service restart.
│
├── redis/
│   └── client.go                        # go-redis/v9 Sentinel-aware Client singleton
│                                        # REQUIRED: use NewFailoverClient, NOT NewClient.
│                                        # Redis Sentinel is a launch gate (see HA section).
│                                        # A single-node NewClient is unacceptable for a
│                                        # financial app — Redis is SPOF for JTI revocation,
│                                        # rate cache, WS fanout, and session state.
│                                        #
│                                        # Implementation:
│                                        #   func NewRedisClient() *redis.Client {
│                                        #     return redis.NewFailoverClient(&redis.FailoverOptions{
│                                        #       MasterName:    "mymaster",
│                                        #       SentinelAddrs: []string{
│                                        #         os.Getenv("REDIS_SENTINEL_1"),  // e.g. "redis-sentinel-1:26379"
│                                        #         os.Getenv("REDIS_SENTINEL_2"),  // e.g. "redis-sentinel-2:26379"
│                                        #         os.Getenv("REDIS_SENTINEL_3"),  // e.g. "redis-sentinel-3:26379"
│                                        #       },
│                                        #       DB:           0,
│                                        #       PoolSize:     20,
│                                        #       DialTimeout:  2 * time.Second,
│                                        #       ReadTimeout:  1 * time.Second,
│                                        #       WriteTimeout: 1 * time.Second,
│                                        #     })
│                                        #   }
│                                        # Env vars: REDIS_SENTINEL_1, REDIS_SENTINEL_2, REDIS_SENTINEL_3
│                                        #   (add all three to env_config_check.sh validation list).
│                                        # REDIS_SENTINEL_* vars replace REDIS_URL (single-node) — see env_config_check.sh.
│                                        # Failover: Sentinel quorum=2 triggers automatic failover
│                                        #   within ~10–30s on primary failure. go-redis/v9 handles
│                                        #   reconnection and master re-discovery transparently.
│                                        # Smoke test assertion (smoke_test.sh):
│                                        #   redis-cli -p 26379 SENTINEL masters | grep -q "mymaster"
│
└── postgres/
    └── pool_factory.go                  # pgx.NewPool per service
                                         # REQUIRED: set MaxConns per service (do not rely on pgx default of 4)
                                         #   gateway: MaxConns=5
                                         #   core:    MaxConns=20
                                         #   pricing: MaxConns=15
                                         #   intelligence: MaxConns=15 (tsvector search + catalog JSONB)
                                         # Also set: MinConns=2, MaxConnLifetime=30m,
                                         #   MaxConnIdleTime=5m, HealthCheckPeriod=1m
                                         # See Infrastructure & Capacity section for full config
```

---

## Observability

```
src/observability/
├── metrics.go                           # prometheus/client_golang — counters/gauges/histograms
│                                        # SLO targets (all achievable at 10k DAU):
│                                        #   p95 < 500ms | p99 < 2000ms  (rates)
│                                        #   p99 < 200ms                  (WS fanout)
│                                        #   error_rate < 0.1% (gateway 5xx / total)
│                                        # ANDROID PERFORMANCE TARGET (aligned — GAP-L1):
│                                        #   cold_start_first_frame Firebase Performance trace: < 80ms
│                                        #   This is the authoritative ceiling; the "50–80ms" range
│                                        #   mentioned in the frontend architecture's timing diagram
│                                        #   is a typical observed window, not a separate specification.
│                                        #   The hard target for QA pass/fail is < 80ms.
│
├── health.go                            # GET /health       → 200 as soon as HTTP server binds
│                                        #                      (DO NOT use for depends_on conditions)
│                                        # GET /health/ready → 503 until ALL of the following are true:
│                                        #   1. DB pool has successfully issued at least one test query
│                                        #   2. Redis connection is reachable (PING)
│                                        #   3. pg NOTIFY listener goroutine has started AND
│                                        #      RebuildProjectionFromDB() catch-up query has completed
│                                        #   Returns 200 only when service is ready to handle traffic.
│                                        # depends_on service_healthy in docker-compose.prod.yml MUST
│                                        #   target /health/ready, not /health, for all 4 services.
│
├── alertmanager/
│   └── alertmanager.yml                 # Alertmanager configuration — runs as a separate container
│                                        # in docker-compose.prod.yml (image: prom/alertmanager).
│                                        # Routes Prometheus alert rules to PagerDuty:
│                                        #
│                                        # global:
│                                        #   resolve_timeout: 5m
│                                        # route:
│                                        #   receiver: pagerduty-critical
│                                        #   group_by: [alertname, severity]
│                                        #   group_wait: 30s
│                                        #   group_interval: 5m
│                                        #   repeat_interval: 4h
│                                        #   routes:
│                                        #     - match: { severity: sev1 }
│                                        #       receiver: pagerduty-critical
│                                        #     - match: { severity: sev2 }
│                                        #       receiver: pagerduty-warning
│                                        # receivers:
│                                        #   - name: pagerduty-critical
│                                        #     pagerduty_configs:
│                                        #       - routing_key: ${PAGERDUTY_KEY}
│                                        #         severity: critical
│                                        #   - name: pagerduty-warning
│                                        #     pagerduty_configs:
│                                        #       - routing_key: ${PAGERDUTY_KEY}
│                                        #         severity: warning
│                                        # docker-compose.prod.yml service entry:
│                                        #   alertmanager:
│                                        #     image: prom/alertmanager:v0.27.0
│                                        #     volumes: [./src/observability/alertmanager:/etc/alertmanager:ro]
│                                        #     command: --config.file=/etc/alertmanager/alertmanager.yml
│                                        #     ports: ["9093:9093"]
│                                        #     restart: always
│                                        # Prometheus prometheus.yml must reference alertmanager:
│                                        #   alerting:
│                                        #     alertmanagers:
│                                        #       - static_configs: [{ targets: ["alertmanager:9093"] }]
│
├── dashboards/
│   ├── latency_dashboard.json           # Grafana: p95/p99 per endpoint; SLO burn-rate
│   ├── error_rate_dashboard.json        # 5xx rate, 429 rate, circuit breaker events
│   ├── ws_dashboard.json                # active WS connections, fanout latency p99
│   └── abuse_dashboard.json             # abuse_detector triggers, IAP anomaly flags
│
└── alerts/
    ├── slo_alerts.yaml                  # p95 > 500ms 5min → SEV-2; error_rate > 0.5% 2min → SEV-1
    ├── infra_alerts.yaml                # OOMKill → SEV-2; disk > 80% → SEV-2; DB lag > 500ms → SEV-2
    └── cost_alerts.yaml                 # Gemini API spend > $200/day → SEV-3; > $500/day → SEV-2
```

---

## Shared

```
src/shared/
├── logger.go                            # slog wrapper; injects { requestID, traceID } from context
├── errors.go                            # TooManyRequestsError, ContentViolationError
├── crypto.go                            # Hash(), GenerateJTI()
├── service_token.go                     # SignServiceToken(secret, timestamp) → HMAC-SHA256 hex
│                                        # VerifyServiceToken(secret, token, timestamp) → bool
│                                        # Used by gateway/middleware/service_token_injector.go
│                                        #   and per-service service_auth.go middleware.
│                                        # All services share INTERNAL_JWT_SECRET env var.
├── rate_limit_policy.go                 # FREE/PREMIUM/ADMIN tier limits; ResolvePolicy(userID)
├── audit_log.go                         # Append(actor, action, entity, metadata)
│
└── types/
    ├── event_envelope.go                # EventEnvelope[T any] generic struct
    ├── pagination.go                    # PaginatedResult[T], CursorPage[T]
    └── api_response.go                  # APISuccess[T], APIError standard shapes
```

---

## Environment Variables

| Variable | Purpose |
|---|---|
| `FIREBASE_PROJECT_ID` | Firebase project ID for server-side ID token verification |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | Firebase Admin SDK service account (separate from `GOOGLE_SERVICE_ACCOUNT_JSON` used for Play) |
| `MSG91_AUTH_KEY` | MSG91 API auth key for OTP send + verify |
| `MSG91_TEMPLATE_ID` | MSG91 DLT-approved OTP SMS template ID (TRAI mandatory for Indian SMS) |
| `MSG91_OTP_EXPIRY_MINUTES` | OTP TTL in minutes; default `10` |
| `OTP_PROVIDER` | `firebase` \| `msg91` \| `both`; default `both` |
| `APP_ENV` | `development` / `staging` / `production` |
| `DATABASE_URL` | PostgreSQL connection string (pgx DSN) |
| `REDIS_SENTINEL_1` | Redis Sentinel node 1 address (e.g. `redis-sentinel-1:26379`) |
| `REDIS_SENTINEL_2` | Redis Sentinel node 2 address |
| `REDIS_SENTINEL_3` | Redis Sentinel node 3 address (tie-breaker) |
| `JWT_PRIVATE_KEY` | RS256 signing key (PEM) |
| `JWT_PUBLIC_KEY` | RS256 verification key (PEM) |
| `INTERNAL_JWT_SECRET` | Service-to-service HMAC-SHA256 token signing (≥64 chars) |
| `GOOGLE_PLAY_PACKAGE_NAME` | IAP validation (Android only) |
| `GOOGLE_SERVICE_ACCOUNT_JSON` | Play Developer API auth |
| `GEMINI_API_KEY` | `pricing`, `intelligence` | Gemini API — server only, never sent to client. `pricing` uses it for AI rate generation (`ai_rate_scheduler_job.go`); `intelligence` uses it for Gemini Vision content moderation (`moderation_client.go`, banner upload only). Both services must be restarted when this key is rotated. |
| `S3_BUCKET` | Shop banner + backup storage |
| `S3_ENDPOINT` | S3-compatible storage endpoint (AWS S3 in production; MinIO for local dev). If using Hetzner Object Storage instead of AWS S3, omit `KMS_KEY_ARN` and use Hetzner's server-side encryption; update `backup_postgres.sh` accordingly. |
| `KMS_KEY_ARN` | AWS KMS key for PostgreSQL backup encryption — **only required when `S3_ENDPOINT` points to AWS S3**. If using Hetzner Object Storage or another S3-compatible provider, remove this var and use provider-native encryption in `backup_postgres.sh`. |
| `PLAY_INTEGRITY_DECRYPTION_KEY` | Play Integrity token verification |
| `CDN_BASE_URL` | CDN base URL (must be HTTPS in production) |
| `SENTRY_DSN` | Error tracking |
| `PAGERDUTY_KEY` | Alertmanager → PagerDuty routing |
| `PROMETHEUS_REMOTE_WRITE_URL` | Metrics export endpoint |

> iOS env vars (`APPLE_BUNDLE_ID`, `APPLE_SHARED_SECRET`) are not included. Add them when adding iOS support alongside `app_store_client.go`.

---

## Operational Runbooks

### Incident Severity Levels

| Severity | Definition | Response | Target Resolution |
|---|---|---|---|
| **SEV-1** | Complete outage, data loss, or security breach | Immediate page 24/7; escalate to Eng Lead in 5min; postmortem within 48h | 30 minutes |
| **SEV-2** | Significant degradation affecting >10% users or revenue | Page; #prod-alerts; status update | 2 hours |
| **SEV-3** | Minor degradation, no immediate user impact | Slack #prod-warnings; investigate within 4h | 24 hours |

---

### Runbook: Pricing Service Failure

**Symptoms:** Circuit breaker open; rates returning 503 or `_degraded: true`.

1. Check container: `docker ps | grep pricing`
2. Verify fallback: `curl .../api/rates/city-mumbai` — expect `200` with `"stale": true`.
3. If crash loop and recent deploy: `docker compose pull pricing && docker compose up -d pricing`
4. Check pg NOTIFY listener lag in pricing logs.
5. After recovery: `bash scripts/warmup_cache.sh`
6. Post timeline to `#incident-sev2`.

---

### Runbook: Redis Failure

**Symptoms:** Redis connection errors across services; rate cache misses; feature flags stale.

**🔴 SEV-1 SECURITY RISK: JTI revocation fails open during Redis outage.**
A user who logged out (JTI added to Redis revocation set) can reuse their access token for up to
15 minutes (the access token TTL) while Redis is down, because the revocation set is unreachable.
Mitigation steps are time-critical:

1. Check container: `docker ps | grep redis`
2. Verify DB fallback: `/api/rates/city-mumbai` should return `200`, not `503`.
3. **🔴 IMMEDIATELY activate `killSwitchWs` and `killSwitchPayments` feature flags** via direct DB update — **in this exact order** (see `scripts/activate_ws_killswitch.sh`):
   ```sql
   -- STEP 1: raise BFF rate limit FIRST (prevents 429 storm when clients start polling)
   UPDATE feature_flags SET value = '60' WHERE key = 'rate_limit_bff_free_rpm';
   -- wait 5s for Redis flag cache to refresh, THEN:
   -- STEP 2: flip kill-switches
   UPDATE feature_flags SET value = 'true' WHERE key IN ('kill_switch_ws', 'kill_switch_payments');
   ```
   This prevents WS connections and payment flows while revocation is unreachable.
   **⚠️ Kill-switch load warning:** Activating `kill_switch_ws` causes all Android clients to fall back to 30-second REST polling against `GET /bff/home`. At 1,200 concurrent users this generates ~40 RPS — matching the normal BFF peak ceiling. **GAP-09 fix: Android clients implement mandatory ±5s jitter (enforced in `HomeScreen.kt` — `Random.nextLong(-5_000L, 5_000L)`) so the load is already spread. Verify the deployed APK version includes this implementation before activating at full DAU; if you are unsure of the client version in the field, treat the load as unspread and proceed with caution.** The rate limit raise in STEP 1 is the automated gate — do not skip it.
4. Restart: `docker compose restart redis`
5. If OOM: `redis-cli CONFIG SET maxmemory 2gb`
6. After restore: `bash scripts/warmup_cache.sh`
7. **Verify JWT revocation is functional:** `bash scripts/smoke_test.sh`
   smoke_test.sh must include a JTI revocation check: logout a test token, confirm subsequent
   request with that token returns 401.
8. Lift kill switches only after smoke_test.sh passes revocation check.
9. **Post-incident:** Any logout issued during the Redis outage window is unenforceable for up to
   15 minutes post-restore. Log affected user IDs from the audit_log and flag for security review.
   Resolution target: **30 minutes** (SEV-1).

---

## CI/CD & Deployment

### CI Pipeline (`.github/workflows/ci.yml`)

Runs on every push to `main`:

1. `lint` — `golangci-lint run` zero warnings
2. `vet` — `go vet ./...`
3. `test` — `go test ./... -race -cover`
4. `build` — `go build ./...`
5. `docker build` — verify all service images build
6. `deploy` — `docker compose pull && docker compose up -d` on VPS via SSH

**Pre-deploy gate** (`scripts/pre_deploy_check.sh`):
```bash
golangci-lint run && go vet ./... && go test ./... -race
bash scripts/env_config_check.sh
```

**Required compliance gate** (add to CI before `go test`, as a fast pre-check):
```bash
# Assert delete_account dual-fire test file exists — compliance invariant.
# Dropping this file is a compliance regression; the gate catches it before tests even run.
test -f test/core/delete_account_usecase_test.go \
  || { echo "FATAL: delete_account_usecase_test.go missing — dual-fire coverage is a compliance invariant"; exit 1; }
# Full test suite (step 3 above) runs the file; no separate re-execution needed.
```

### Environment Configuration

| Overlay | Config |
|---|---|
| `dev` | `docker-compose.yml` — relaxed limits, local postgres/redis |
| `staging` | `docker-compose.prod.yml` on staging VPS |
| `production` | `docker-compose.prod.yml` on prod VPS (Hetzner CPX41 or equivalent) |

### Upgrade Path to Kubernetes

When any single service needs >2 replicas sustainably, migrate that service first:
1. Add `k8s/` directory with Deployment + Service + HPA for that service only.
2. Keep remaining services on Docker Compose until they also need scaling.
3. Full K8s migration at ~50k DAU or when operational complexity justifies it.

---

## Compliance

### Account Deletion Flow (`DELETE /user/account`)

1. Extract `userID` from JWT `sub` claim. (`DELETE /user/account` carries no `:userID` path parameter — the authenticated caller's identity is the only input.)
2. Check account is not already deleted.
3. Soft-delete user row (`deleted_at = NOW()`).
4. Revoke all active JTIs for the user.
5. pg NOTIFY `account_deleted` (consumed by intelligence to purge catalog/shop data).
6. Schedule hard-delete after **30-day grace period**.
7. Write audit entry.
8. Returns `204 No Content`.

### Consent Logging

`POST /user/consent` — **Idempotent:** same `userID + type + version` → returns existing record.
Consent types: `privacy_policy | tos`
Table is insert-only: `REVOKE UPDATE, DELETE ON consent_log FROM app_role`.
**GAP-08 fix:** Any `consentType` value other than `"privacy_policy"` or `"tos"` (including `"ai_disclaimer"`) is rejected by `log_consent_usecase.go` with `HTTP 400 { "error": "invalid_consent_type" }` before the DB insert. See `log_consent_usecase.go` for the allowlist guard implementation.

### Data Retention Policy

| Data | Retention | Action |
|---|---|---|
| `ai_rate_snapshots` | 30 days | DELETE |
| `flag_audit` rows | 1 year | DELETE |
| Soft-deleted users | 30-day grace | Hard DELETE cascade |
| `audit_log` (non-billing) | 2 years | ARCHIVE to S3, then DELETE |
| `audit_log` (billing) | 7 years | RETAIN — financial compliance |
| `receipt_log` | 7 years | RETAIN — financial compliance |
| `consent_log` | Forever | RETAIN — legal evidence |

---

## Event Flow Summary (pg LISTEN/NOTIFY)

| Event (pg channel) | Notifier | Listener(s) |
|---|---|---|
| `user_created` | core | core (provision free subscription) |
| `user_banned` | core | pricing (close WS connection) |
| `account_deleted` | core ¹ | intelligence (purge catalog/shop data) |
| `rate_updated` | pricing | pricing WS fanout (Redis pub/sub) |
| `rate_stale` | pricing | Alertmanager only |
| `subscription_activated` | core | pricing (update projection), intelligence (update projection) |
| `subscription_expired` | core | pricing (update projection), intelligence (update projection) |
| `alert_delivered` | core | pricing WS (push to client) |
| `flag_updated` | core | gateway (Redis cache invalidation) |
| `ai_rate_snapshot_ready` | pricing | pricing WS (push to rates channel) |

> ¹ `account_deleted` fires from **two** sources:
> - `delete_account_usecase.go` — user-initiated deletion (immediate, on `DELETE /user/account`).
> - `hard_delete_job.go` — system-initiated hard-delete after the 30-day grace period expires.
> The listener in `intelligence/events/listeners.go` (`DELETE FROM shops / invoices WHERE user_id = $1`)
> is naturally idempotent — a double-fire on the same `userID` is safe. No special guard needed.
> Test coverage: `delete_account_usecase_test.go` must cover both fire paths.

> Design view tracking uses Redis INCR counters (`design:views:{designID}`) flushed to the DB every 5 minutes by `flush_view_counts_job.go`.

---

## Port Map

| Service | Port | Runtime |
|---|---|---|
| gateway | 4000 | Go |
| core | 4001 | Go |
| pricing | 4002 | Go |
| intelligence | 4003 | Go |
