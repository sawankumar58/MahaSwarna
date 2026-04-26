# MahaSwarna — Architecture

> **Scope:** Go backend (4 services) + Kotlin Android client.
> Optimised for **10k DAU** on a single Hetzner CPX41 node (~$60–80/month), with a documented upgrade path to Kubernetes at 50k DAU.

---

## Table of Contents

- [Technology Choices](#technology-choices)
- [System Overview](#system-overview)
- [Infrastructure Sizing](#infrastructure-sizing)
- [Backend Services](#backend-services)
- [Android Client](#android-client)
- [Data Architecture](#data-architecture)
- [Event Bus](#event-bus)
- [Cross-Cutting Invariants](#cross-cutting-invariants)
- [Architecture Decision Records](#architecture-decision-records)
- [Upgrade Path](#upgrade-path)

---

## Technology Choices

### Backend

| Concern | Choice | Rationale |
|---|---|---|
| Runtime | Go 1.23 | Single binary per service, low memory footprint, fast startup |
| HTTP router | `net/http` + `chi` | Lightweight and idiomatic Go |
| WebSocket | `gorilla/websocket` | Native Go; no CGo dependency |
| Auth algorithm | JWT RS256 | Asymmetric — public key is safely distributable across services |
| OTP (primary) | Firebase Phone Auth | Client-side flow; highly reliable on Indian Android devices |
| OTP (fallback) | MSG91 SMS | Tier-1 Indian SMS gateway; TRAI DLT-compliant |
| Database | PostgreSQL + `pgx/v5` | Single instance + 1 read replica; per-service schema isolation |
| Async bus | PostgreSQL `LISTEN/NOTIFY` | Zero additional infrastructure; adequate throughput at 10k DAU |
| Cache | Redis Sentinel (3-node) | JTI revocation, rate cache, WS fanout, session state |
| AI | Google Gemini API | Sole rate source; proxied server-side — key never reaches the client |
| IAP | Google Play Developer API | Android-only; iOS is not supported |
| Integrity | Google Play Integrity API | Required on login and before any purchase |
| Object storage | S3-compatible | Shop banners and PostgreSQL backups |
| Monitoring | Prometheus + Grafana + Loki | SLO dashboards and structured log aggregation |
| Error tracking | Sentry | 5xx capture with trace correlation |
| CI/CD | GitHub Actions | lint → vet → test -race → build → deploy |

### Android

| Concern | Choice | Notes |
|---|---|---|
| Language | Kotlin 2.2.20 | |
| UI | Jetpack Compose + Material 3 | |
| DI | Hilt | |
| Local DB | Room 2.8.3 | `fallbackToDestructiveMigration()` is **banned** |
| HTTP | Retrofit 3.0.0 + OkHttp 5 (`okhttp-android`) | Built-in `kotlinx.serialization`; no separate converter artifact |
| WebSocket | OkHttp 5 `WebSocket` API | Same artifact as the HTTP client |
| Auth storage | EncryptedSharedPreferences (AES-256) | Plain SharedPreferences are prohibited |
| IAP | Play Billing Library 7 (`billing-ktx`) | `-ktx` is intentional; required for coroutine API |
| Images | Coil 3 | |
| Charts | Vico `compose-m3` 2.x | Rate history line chart |
| PDF (Diary export) | `android.graphics.pdf.PdfDocument` (platform API) | Zero license risk; iTextG (AGPL) is banned |
| Firebase BOM | 34.0.0 | **No `-ktx` suffix** — BOM 34 bundles Kotlin extensions natively |
| Serialisation | `kotlinx.serialization-json 1.7.3` | |
| Paging | Paging 3 + `RemoteMediator` | Catalog infinite scroll with Room offline cache |

---

## System Overview

```
┌──────────────────────────────────────────────────────┐
│         Native Android Client  (MahaSwarna)           │
│  Kotlin · Compose · Hilt · Room · OkHttp 5 · Coil   │
│  WebSocket (OkHttp) · Retrofit 3 · Play Billing 7   │
└───────────────────────────┬──────────────────────────┘
                            │ HTTPS / WSS
┌───────────────────────────▼──────────────────────────┐
│            API Gateway  :4000  (Go / chi)             │
│  TLS termination · rate limiting · circuit breakers   │
│  JWT pre-validation · BFF aggregation · feature flags │
│  abuse detection · idempotency · service token inject │
└──────┬────────────────┬────────────────┬─────────────┘
       │                │                │
  :4001 core       :4002 pricing    :4003 intelligence
  auth/identity    rates/WS         catalog/marketplace
  billing/IAP      Gemini AI        invoices/shop
  alerts/push      rate watchdog    content moderation
  feature flags
       │                │                │
       └────────────────┴────────────────┘
                        │
           ┌────────────▼─────────────┐
           │       PostgreSQL 15       │
           │   per-service schemas     │
           │   LISTEN/NOTIFY event bus │
           └────────────┬─────────────┘
                        │
           ┌────────────▼─────────────────────┐
           │   Redis Sentinel  (3-node)         │
           │   primary + replica + tie-breaker  │
           │   rate cache · JTI revocation      │
           │   WS fanout · session state        │
           └───────────────────────────────────┘
```

---

## Infrastructure Sizing

Validated load targets: **10k DAU**, peak concurrent ~1,200–1,500 (9–11 AM IST burst).

### Load Estimates

| Metric | Value | Basis |
|---|---|---|
| Peak concurrent users | ~1,200–1,500 | 12–15% of DAU simultaneously active |
| Peak concurrent WebSocket connections | ~600–900 | ~60% of concurrent users hold a live WS |
| REST API peak RPS | ~80–120 | 1,200 users × ~4 req/min ÷ 60 |
| BFF `/bff/home` peak RPS | ~25–40 | Launch burst during morning market open |
| DB concurrent connections needed | ~60–80 | 4 services × 15–20 pool slots each |
| Redis peak ops/sec | ~5,000–8,000 | WS fanout + cache reads + rate limiter |

### VPS Sizing — Hetzner CPX41 (8 vCPU, 16 GB RAM)

| Resource | 10k DAU Peak | CPX41 Capacity | Headroom |
|---|---|---|---|
| CPU (all 4 Go services) | ~2–3 vCPU | 8 vCPU | 2.5–3× spare |
| RAM (services + PG + Redis) | ~6–8 GB | 16 GB | ~2× spare |
| Network (rates + WS) | ~30–50 Mbps | 1 Gbps | 20× spare |

> Enable swap (4 GB via `fallocate`) as a safety valve against Redis OOM events.

### PostgreSQL Connection Pool

> **Critical:** `pgx` defaults to 4 connections when `MaxConns` is unset. Without explicit tuning, this causes connection starvation under load.

| Service | MaxConns | Notes |
|---|---|---|
| `gateway` | 5 | Redis-primary; minimal PG use (idempotency log only) |
| `core` | 20 | Auth, billing, alerts — the most write-heavy service |
| `pricing` | 15 | Rate reads (mostly Redis), WS state |
| `intelligence` | 15 | tsvector full-text search + catalog JSONB queries |
| **Total** | **55** | Within PG default `max_connections = 100` |

Required `postgresql.conf`:

```ini
max_connections      = 150
shared_buffers       = 4GB    # 25% of 16 GB RAM
effective_cache_size = 8GB
work_mem             = 16MB
```

### Redis Memory Budget

```
WS connection registry:    ~600 keys × 1 KB  =   0.6 MB
Rate cache (61 cities):    61 × 2 KB          =   0.1 MB
BFF shared rate cache:     61 cities × 2 KB   =   0.1 MB   (city-scoped, not per-user)
BFF alert cache:           ~200 × 1 KB        =   0.2 MB   (users with active alerts only)
Session / JTI revocation:  10,000 × 0.5 KB   =   5.0 MB
Feature flags:             ~10 KB             =   trivial
Rate limiter counters:     1,500 × 100 B      =   0.2 MB
Catalog rec cache:         50 regions × 5 KB  =   0.3 MB
Design view counters:      ~5,000 × 50 B      =   0.25 MB
─────────────────────────────────────────────────────────
Total active data:                             ~6.8 MB
+ Redis overhead + fragmentation:             ~40 MB peak
```

Redis 2 GB allocation (`maxmemory 2gb`, policy `allkeys-lru`) provides substantial headroom.

### High-Availability Requirements

Redis Sentinel is a **launch gate** — a single Redis node is unacceptable for a financial pricing application at any DAU. Redis is a single point of failure for JTI revocation, rate cache, WS fanout, and session state.

**Minimum production Redis configuration:**

```
redis-primary:   CX22  (redis-server, primary)
redis-replica:   CX22  (redis-server + sentinel, REPLICAOF primary)
redis-sentinel:  CX11  (sentinel only — quorum tie-breaker)
```

- `go-redis/v9` uses `redis.NewFailoverClient()` — **never** `NewClient()`
- Sentinel quorum = 2; automatic failover in ~10–30s on primary failure
- All three `REDIS_SENTINEL_*` env vars are validated in `env_config_check.sh`

---

## Backend Services

### Gateway — Port 4000

The gateway is the single public-facing entrypoint for all REST traffic. It never touches business logic — it routes, validates, and aggregates.

**Middleware chain (applied in order):**

```
RequestID → TraceContext → GlobalRateLimiter → JwtPreValidator →
FeatureFlags → ServiceTokenInjector → Idempotency → AbuseDetector
```

**BFF Aggregation (`/bff/home`):**

The home endpoint is assembled inline in the gateway using a two-key Redis cache split:

| Cache Key | TTL | Scope |
|---|---|---|
| `home:shared:{cityID}` | 30s | Shared across all users in the same city |
| `home:alerts:{userID}` | 30s | Per-user; skipped if the user has zero active alerts |

Target: BFF response < 1,500ms. All upstream calls use an 800ms per-upstream context deadline; partial degradation is served with `_degraded: true`.

### Core — Port 4001

Auth/identity, billing, alerts, and feature flags.

**OTP — dual-provider architecture:**

| Provider | Verification Flow | When Used |
|---|---|---|
| Firebase Phone Auth (primary) | Client-side SMS trigger → Firebase ID token → backend verifies via Admin SDK | Default (`OTP_PROVIDER = firebase` or `both`) |
| MSG91 SMS (fallback) | Backend sends OTP via MSG91 REST API → client submits code to `/auth/login` | `OTP_PROVIDER = msg91` or Firebase failure in `both` mode |

**JWT configuration:**

- Algorithm: RS256 (private key signs; public key verifies)
- Access TTL: **15 minutes** · Refresh TTL: **30 days** (stored in DB, revocable via JTI)
- Payload claims: `sub`, `jti`, `tier` (`FREE | PREMIUM | ADMIN`), `region`, `iat`, `exp`
- `region` is set at login from `users.city_id` — no DB lookup on each request
- Multi-key acceptance required for zero-downtime key rotation (see [`RUNBOOK.md`](RUNBOOK.md))

**Play Integrity on login:**

`POST /auth/login` requires `integrityToken` in the request body, verified server-side before any JWT is issued. On failure: `HTTP 403 { "error": "device_not_trusted" }`. This prevents rooted or emulated devices from consuming live rates indefinitely on the FREE tier.

### Pricing — Port 4002

Gold/silver rate engine, WebSocket server, and Gemini AI scheduling.

**Gemini failure handling — degradation policy:**

| Condition | Action |
|---|---|
| Single city timeout (2s deadline) | Serve last snapshot with `stale: true`; emit `rate_stale` NOTIFY |
| Sanity check fails (> 2% delta from previous) | Reject new snapshot; serve previous with `stale: true`; Sentry SEV-2 |
| 3 consecutive full-run failures | Activate `kill_switch_ws` flag; escalate to PagerDuty SEV-1 |

> The sanity threshold is **2%** (not 5%). Gold intraday moves are typically 0.3–1.0%; a 5% gate would silently accept ₹300/gram errors on a ₹6,000/gram price. Configurable via the `rate_sanity_threshold_pct` feature flag without a deploy.

**Rate schedule:** `cron("0 10-19 * * 1-6", timezone="Asia/Kolkata")` — must be initialised with IST. `robfig/cron` defaults to UTC; without an explicit timezone, the window fires 5.5 hours late.

**WebSocket graceful shutdown:**

`pricing/main.go` implements a two-phase drain on `SIGTERM`: (1) stop accepting new WS upgrades via `srv.Shutdown(ctx)` with a 15s grace period; (2) send close frames to all live connections via `connectionRegistry.CloseAll()`. Without this, every deploy causes a reconnect storm. `docker-compose.prod.yml` sets `stop_grace_period: 20s` for the pricing service.

### Intelligence — Port 4003

Jewellery catalog, marketplace, and PDF invoice generation.

**Invoice rate source resolution (priority order):**

| Priority | Condition | Rate Source |
|---|---|---|
| 1 | `GoldRateOverride > 0` | `client_override` |
| 2 | `snapshot.Source == "manual_override"` | `manual_override` |
| 3 | `snapshot.Stale == true` | `stale` |
| 4 | Default | `live` |

The client shows a warning for any source other than `"live"`. Unknown future source values are treated as `"stale"` (future-proof).

**PDF generation:** `signintech/gopdf` (pure Go, no CGo). Noto Sans font for Devanagari support. PDFs are **not stored server-side** — bytes are returned directly in the API response and the client persists them to local storage. Invoice records in the DB store only `pdf_size_bytes` for audit purposes.

> `POST /catalog/image-search` is intentionally absent from `router.go`. The Android kill-switch `killSwitchImageSearch` defaults to `true`. Both backend and Android must ship simultaneously when this feature launches.

---

## Android Client

### Cold Start Timing Budget

```
T+0ms     OS applies SplashScreen API (zero Compose frames)
T+5ms     Application.onCreate():
            NotificationChannelSetup.createChannels()   ← BEFORE Firebase
            Hilt builds app component (NetworkModule, DatabaseModule, WsModule)
            Room.openAsync()  [non-blocking]
            Firebase.initializeApp()  [async, off critical path]
            NOTE: TokenStore is NOT accessed here
T+5ms     SplashScreen routing from token_exists_marker plain file (zero Keystore access)
T+10ms    MainActivity.setContent{} → HomeScreen()
T+10ms    RatesViewModel.init() → ratesRepository.getCachedRates()  [Room, ~5–15ms]
T+80ms    ← FIRST MEANINGFUL RENDER from Room cache ✅
T+80ms    Background coroutines: feature flags, BFF refresh, JWT pre-warm
T+800ms   WsClient.connect() (token guaranteed valid ≥ 12 minutes)
T+900ms   WS JWT auth handshake
T+1000ms  WS subscribed to rates|alerts channels
T+1200ms  BFF response → Room update → Flow re-emit → Compose recompose
```

**First render target: 50–80ms ✅  (400ms budget — 5× headroom)**

### OkHttpClient Interceptor Order (mandatory)

```
1. VersionInterceptor       — Accept-Version: v1; HTTP 410 → VersionDeprecated; never retried
2. AuthInterceptor          — Bearer token; 401 → refresh + single retry; synchronized(refreshLock)
3. AiQuotaInterceptor       — reads X-Ai-Quota-* response headers → PreferenceStore; pass-through
4. LogRedactionInterceptor  — strips Authorization + Set-Cookie from logs
5. HttpLoggingInterceptor   — debug builds only
```

> `@Named("s3")` OkHttpClient for presigned S3 uploads **must not** include `AuthInterceptor` — presigned URLs reject the `Authorization` header.

### Room Entity Ownership

| Entity | Canonical Package | Notes |
|---|---|---|
| `RateEntity` | `data/local/entity/` | |
| `HomeEntity` | `data/local/entity/` | |
| `AlertEntity` | `data/local/entity/` | |
| `DesignEntity` | `data/local/entity/` | |
| `BillEntity` + `BillFts` | `feature/diary/data/local/` | Stub exists at `data/local/entity/BillEntity.kt` — do not implement |
| `CustomerEntity` + `CustomerFts` | `feature/diary/data/local/` | Stub exists at `data/local/entity/CustomerEntity.kt` — do not implement |
| `LedgerEntryEntity` | `feature/diary/data/local/` | Stub exists at `data/local/entity/LedgerEntryEntity.kt` — do not implement |

`AppDatabase` imports all entities from their canonical packages.

### HomeViewModel Init Order (invariant)

```kotlin
// Step 1 — shimmer timeout guard (MUST be FIRST — assigned before Room collector runs)
shimmerJob = viewModelScope.launch {
    delay(2_000)
    if (_uiState.value is Loading) _uiState.value = NoDataAvailable
}

// Step 2 — Room cache read (launched after shimmerJob is assigned)
viewModelScope.launch {
    homeRepository.getCachedHome().collect { cached ->
        if (cached != null && _uiState.value is Loading) {
            _uiState.value = Success(cached)
            shimmerJob?.cancel()
        }
    }
}

// Steps 3–5 in a single coroutine
viewModelScope.launch {
    // Step 3: JWT pre-warm (wrapped in try/catch — uncaught exception cancels connect)
    val remaining = sessionManager.accessTokenRemainingMs()
    if (remaining < 3 * 60_000L) {
        try { authRepository.refreshToken() }
        catch (e: Exception) { Crashlytics.log("JWT pre-warm failed: ${e.message}") }
    }
    // Step 4: WebSocket connect
    wsClient.connect(tokenStore.getAccessToken())
    // Step 5: observe live data
    observeHomeData().collect { data ->
        shimmerJob?.cancel()
        _uiState.value = Success(data)
    }
}
```

### WS Kill-Switch — Polling Fallback

When `killSwitchWs == true`, the app falls back to 30-second REST polling:

```kotlin
// HomeScreen.kt — mandatory ±5s jitter to prevent thundering herd
lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {
    while (true) {
        delay(30_000L + Random.nextLong(-5_000L, 5_000L))
        homeRepo.refresh()
    }
}
```

Polling mode generates ~40 RPS at 1,200 concurrent users — matching normal BFF peak. The backend team must raise the FREE-tier BFF rate limit before activating this kill-switch at scale.

---

## Data Architecture

### PostgreSQL Schema Isolation

Each service owns its own PostgreSQL schema. Cross-schema queries are **prohibited** — all cross-service reads use the internal HTTP API (e.g., `GET http://core:4001/internal/subscriptions/active` with `X-Service-Token`).

### Room as Launch Source of Truth

Android always renders from Room on cold start:

1. `RatesRepository` emits the last cached rate instantly
2. A BFF REST call refreshes in the background; on success, **all fields** are persisted to Room
3. WebSocket takes over for live updates once connected
4. `isStale` is derived from the backend's `rate.stale` field — **never** computed from `cachedAt`

### Diary — Local-Only Guarantee

No Diary data (bills, ledger entries, customers) is ever transmitted to the backend. Room is the sole store. `clearSessionData()` (called on logout) must not touch Diary tables — only `clearAll()` (called from `DeleteAccountUseCase` after confirmed server 204) wipes Diary.

### AppDatabase Migration Policy

`fallbackToDestructiveMigration()` is **banned**. Every schema bump requires an explicit `Migration`:

```kotlin
val MIGRATION_1_2 = object : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE BillEntity ADD COLUMN ...")
    }
}
```

Every migration must include a `@Test` asserting that Diary row counts are preserved before and after.

---

## Event Bus

PostgreSQL `LISTEN/NOTIFY` is the async event bus.

| Channel | Notifier | Listener(s) | Notes |
|---|---|---|---|
| `user_created` | `core` | `core` | Self-listener to provision a free subscription |
| `user_banned` | `core` | `pricing` | Force-disconnects the user's WS connection |
| `account_deleted` | `core` ¹ | `intelligence` | Purges shops and invoices |
| `rate_updated` | `pricing` | `pricing` WS fanout (Redis pub/sub) | |
| `rate_stale` | `pricing` | Alertmanager only | SEV-2 alert |
| `subscription_activated` | `core` | `pricing`, `intelligence` | Updates the subscription projection |
| `subscription_expired` | `core` | `pricing`, `intelligence` | Updates the subscription projection |
| `alert_delivered` | `core` | `pricing` WS | Pushed to client |
| `flag_updated` | `core` | `gateway` | Redis cache invalidation |
| `ai_rate_snapshot_ready` | `pricing` | `pricing` WS | Pushes to the rates channel |

> ¹ `account_deleted` fires from two sources: `delete_account_usecase.go` (user-initiated) and `hard_delete_job.go` (system-initiated after the 30-day grace period). The intelligence listener is naturally idempotent — deleting already-absent rows is a no-op. Both fire paths must be covered in `delete_account_usecase_test.go`.

**NOTIFY reconnect invariant:** Every `pgnotify.NewListener` call must accept an `onReconnect` callback that re-runs the startup catch-up query on every reconnection, not only at startup. PostgreSQL NOTIFY is fire-and-forget — any event emitted during a reconnect window is permanently lost.

---

## Cross-Cutting Invariants

> These invariants apply across the entire codebase. Deviating from any of them is a **bug**, not a trade-off.

### Security

- JWT, receipt tokens, and API keys are **never** written to any log at any level
- All 5xx errors are captured to Sentry before the response is sent
- Play Integrity is verified before any purchase-related endpoint executes, and on login
- No endpoint trusts client-provided purchase status — the DB subscription record is the sole source of truth
- Service-to-service calls use `X-Service-Token: HMAC-SHA256(timestamp + INTERNAL_JWT_SECRET)` — never the user's JWT

### Data Integrity

- All environment variables are validated at startup — missing secrets cause `os.Exit(1)`
- `rate.isStale` is mapped from the backend signal — never computed from `cachedAt`
- AI quota values are sourced from `PreferenceStore` written by `AiQuotaInterceptor` via response headers, not response body fields
- `_degraded: true` in a BFF response is a transient delivery signal — not persisted to Room

### Room (Android)

- `fallbackToDestructiveMigration()` is banned
- `clearSessionData()` clears only `RateEntity`, `HomeEntity`, `AlertEntity`, `DesignEntity` — never Diary tables
- `clearAll()` (full wipe including Diary) is called only from `DeleteAccountUseCase` after server 204
- Every schema bump requires an explicit `Migration` + a `@Test` asserting Diary row counts

### Navigation (Android)

- `navController` is hoisted in `MainActivity.setContent`, not inside `AppNavGraph`
- HTTP 410 → `UpdateRequiredScreen` (non-dismissible, back-nav disabled) — handled before any other error path and never retried

---

## Architecture Decision Records

### ADR-001 — Invoice PDF Wire Format ✅ DECIDED

**Decision:** JSON wrapper with base64-encoded PDF bytes.

**Wire format:**

```json
{
  "invoice_id":   "uuid-string",
  "pdf_bytes":    "<base64-encoded PDF>",
  "generated_at": "2025-01-15T10:30:00+05:30",
  "rate_source":  "live | stale | client_override | manual_override"
}
```

- **Go:** `PdfBytes []byte` in the response struct is base64-encoded automatically by `encoding/json`
- **Kotlin:** `pdfBytes: ByteArray` in `@Serializable InvoiceResponse` is decoded automatically by `kotlinx.serialization`. Retrofit return type is `InvoiceResponse` — not `ResponseBody`

**Rationale:** Invoice PDFs at this scale are < 500 KB. Base64 overhead (~33%) is negligible, and a single JSON response keeps all fields atomic. Switch to streaming only if PDF size exceeds 5 MB and OOM is observed on budget devices.

**Scope:** Any deviation from this ADR in either codebase is a breaking contract mismatch. `invoice_handler.go` and `InvoiceDto.kt` are the two canonical locations.

---

### ADR-002 — WebSocket Gateway Bypass ✅ DECIDED

**Decision:** Port `:4002` (pricing/WebSocket) is publicly exposed; clients connect directly, bypassing gateway middleware.

**Rationale:** Routing WS upgrades through the gateway adds a second TCP hop and a goroutine per connection. At 600–900 concurrent WS connections, this doubles memory overhead without meaningful security benefit — the gateway's JWT pre-validator adds nothing beyond what `ws_server.go`'s own auth handshake already provides.

**Accepted trade-offs and required compensating controls:**

- `ws_server.go` self-enforces: JWT auth handshake, WS handshake rate limit (20 new upgrades/IP/min via Redis), `http.MaxBytesReader` on the upgrade request body, TLS-only enforcement in production
- Hetzner firewall restricts `:4002` to inbound TCP only
- `smoke_test.sh` asserts that plain HTTP to `:4002` returns `403`

> Do not re-route WS through the gateway without profiling memory impact first. If the gateway is ever extracted to a separate VPS, revisit — the hop cost becomes negligible at that point.

---

### ADR-003 — Secret Management Upgrade Path ✅ DECIDED

**Launch:** `.env.production` encrypted at rest via `age-encryption`. Key stored offline.

**Post-launch (~30 days):** Migrate to HashiCorp Vault OSS (self-hosted on the VPS as a Docker service):

- Per-service Vault policies (`core` cannot read `pricing` keys)
- Audit log of every secret read (Vault audit backend → Loki)
- Automatic rotation for `INTERNAL_JWT_SECRET` and DB passwords
- Each service reads secrets via `VAULT_TOKEN` at boot; renewal via vault-agent sidecar

**Never store secrets in:** git history, Docker image layers, container env vars in `docker-compose.yml` (use the `secrets:` block), or Sentry/Grafana logs.

---

## Upgrade Path

Extract services in this order when outgrowing the current architecture:

| DAU Threshold | Trigger | Recommended Action |
|---|---|---|
| ~50k | Any single service needs > 2 replicas | Extract `pricing` + WebSocket first — highest traffic, easiest to isolate |
| ~50k | `pg_stat_activity` wait events > 5% of queries | Add PG read replica (already designed) |
| ~100k | PG `NOTIFY` becoming a bottleneck | Migrate to Kafka |
| Multi-region | Need > 3 replicas of any service | Full Kubernetes migration |
| Any | Redis memory > 60% of `maxmemory` | Increase to 4 GB or move to managed Redis |
| Any | WS concurrent connections > 5,000 | Shard `connection_registry.go` + add a second pricing node |

The `src/contracts/` package and per-service schema layout make this migration mechanical — there are no cross-schema dependencies to untangle.
