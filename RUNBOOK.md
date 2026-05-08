# MahaSwarna — Runbook

> **Purpose:** Operational procedures for the MahaSwarna production environment. Covers deployment, incident response, secret rotation, database operations, and scaling triggers.
>
> **Keep this document current** — stale runbooks cost time during incidents.

---

## Table of Contents

- [Incident Severity Levels](#incident-severity-levels)
- [Pre-Deploy Checklist](#pre-deploy-checklist)
- [Deployment Procedure](#deployment-procedure)
- [Post-Deploy Verification](#post-deploy-verification)
- [Runbook: Gemini Outage](#runbook-gemini-outage)
- [Runbook: Pricing Service Failure](#runbook-pricing-service-failure)
- [Runbook: Redis Failure](#runbook-redis-failure)
- [Runbook: Database Issues](#runbook-database-issues)
- [Runbook: WebSocket Disruption](#runbook-websocket-disruption)
- [Runbook: Play Billing Issues](#runbook-play-billing-issues)
- [Secret Rotation](#secret-rotation)
- [Database Operations](#database-operations)
- [Scaling Triggers](#scaling-triggers)
- [Environment Variables Reference](#environment-variables-reference)

---

## Incident Severity Levels

| Severity | Definition | Response SLA | Resolution Target |
|---|---|---|---|
| 🔴 **SEV-1** | Complete outage, data loss, or security breach | Immediate page 24/7; Eng Lead notified within 5 min | 30 minutes |
| 🟡 **SEV-2** | Significant degradation affecting > 10% of users or revenue | Page; `#prod-alerts`; status update every 30 min | 2 hours |
| 🟢 **SEV-3** | Minor degradation with no immediate user impact | Slack `#prod-warnings`; investigate within 4h | 24 hours |

Post-mortem required within 48h for any SEV-1.

---

## Pre-Deploy Checklist

Run before every production deployment:

```bash
# 1. Backend pre-deploy gate
#    Covers: migration dry-run, golangci-lint, go vet, go test -race, JWT round-trip, Redis ping
bash scripts/pre_deploy_check.sh

# 2. Validate all required env vars are present and well-formed
bash scripts/env_config_check.sh

# 3. Compliance gate (also enforced in CI)
test -f test/core/delete_account_usecase_test.go \
  || { echo "FATAL: delete_account_usecase_test.go missing — compliance invariant"; exit 1; }

# 4. Confirm Redis Sentinel is healthy
redis-cli -p 26379 SENTINEL masters | grep -q "mymaster" \
  || { echo "FATAL: Redis Sentinel not healthy — do not deploy"; exit 1; }

# 5. Confirm DB migrations are backward-compatible
#    pre_deploy_check.sh runs the dry-run; review output manually for breaking changes
#    (no DROP COLUMN, no NOT NULL without DEFAULT)
```

---

## Deployment Procedure

### Standard Deploy (Backend)

```bash
# Pull latest images and restart services in dependency order
docker compose pull
docker compose up -d postgres redis-primary redis-replica redis-sentinel
sleep 10  # allow DB + Redis to reach healthy state

docker compose up -d core
sleep 20  # wait for core /health/ready (subscription projection catch-up)

docker compose up -d pricing intelligence
sleep 30  # wait for pricing + intelligence /health/ready (internal API call to core)

docker compose up -d gateway

# Verify all services report ready
bash scripts/smoke_test.sh
```

> **`start_period` note:** `docker-compose.prod.yml` sets `start_period: 300s` for `pricing` and `intelligence` — they call `GET /internal/subscriptions/active` on core with up to 8 retry attempts (exponential backoff, max ~4.25 min). Do not route traffic to these services until `/health/ready` returns `200`.

### Android Release

Triggered automatically on `v*` tags via `.github/workflows/release.yml`:

```
ktlint + Detekt → ./gradlew test → ./gradlew connectedCheck → ./gradlew bundleRelease
→ Sign AAB (keystore secrets in GitHub Actions) → Upload to Play Store internal track
```

Promote from internal track to production via Play Console after manual QA sign-off.

### Cache Warm-Up (Required After Every Deploy)

```bash
bash scripts/warmup_cache.sh
```

This script fetches rates for all 61 cities in parallel, ensuring Redis rate keys are hot before the gateway routes traffic. The `cache_warmer` sidecar in `docker-compose.prod.yml` runs this automatically on every pricing container restart.

---

## Post-Deploy Verification

```bash
# Full smoke test suite
bash scripts/smoke_test.sh
```

`smoke_test.sh` asserts all of the following:

- All 4 services return `200` on `/health/ready` (not just `/health`)
- JWT issue + verification round-trip succeeds
- `GET /rates/mumbai` returns a valid rate with the `stale` field present
- Redis Sentinel responds: `redis-cli -p 26379 SENTINEL masters | grep -q "mymaster"`
- Redis eviction policy: `redis-cli CONFIG GET maxmemory-policy` returns `allkeys-lru`
- WebSocket connect to `:4002` succeeds
- Plain HTTP to `:4002` returns `403` (TLS enforcement)
- JTI revocation: logout a test token → confirm subsequent request returns `401`

### SLO Targets for Production

| Metric | Target |
|---|---|
| p95 latency (rates) | < 500ms |
| p99 latency (rates) | < 2,000ms |
| p99 WS fanout latency | < 200ms |
| Gateway 5xx error rate | < 0.1% |

---

## Runbook: Gemini Outage

**Symptoms:** Rates not updating; `stale: true` on all cities; Sentry SEV-2 from `rate_stale` NOTIFY; PagerDuty page after 3 consecutive scheduler failures.

### Step 1 — Assess

```bash
# Check Gemini API reachability
curl -sf "https://generativelanguage.googleapis.com/v1/models" \
  -H "x-goog-api-key: $GEMINI_API_KEY" | jq .

# Check pricing service logs
docker logs mahaswarna_pricing --tail=100 | grep -i "gemini\|snapshot\|stale"

# Check rate quality watchdog status in Sentry
```

### Step 2 — Confirm Automatic Stale-Rate Serving

The pricing service automatically serves the last known snapshot with `stale: true` during a Gemini outage. No manual action is required for the first few hours — users see a stale banner but rates do not go to zero.

### Step 3 — Activate Kill Switch (if > 1 IST Market Session Affected)

> **Use the canonical script — do not run the SQL manually.** The script raises the FREE-tier BFF rate limit *before* flipping the flag, then waits 5 seconds for the Redis flag cache to expire, then activates the kill-switch. Skipping the rate-limit pre-raise causes a 40 RPS polling spike against `GET /bff/home` at peak DAU, producing a simultaneous HTTP 429 storm for all users. See PRD OQ-8 for full rationale.

```bash
# Canonical kill-switch activation — handles all three steps atomically:
#   STEP 1: raise rate_limit_bff_free_rpm to 60
#   STEP 2: sleep 5s for Redis flag cache to refresh
#   STEP 3: flip kill_switch_ws
bash scripts/activate_ws_killswitch.sh
```

> **Manual pre-check required before running the script at full DAU:** confirm that the deployed APK version on Play Console includes the `±5s jitter` polling implementation (`HomeScreen.kt`). If the APK version cannot be confirmed, treat the polling load as unspread and exercise extra caution.

### Step 4 — Manual Rate Override (Emergency)

If Gemini is down for an extended period and rates must be updated manually:

```bash
# Replace $CITY, $GOLD, $SILVER with actual values (in INR per gram)
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
INSERT INTO ai_rate_snapshots (city_id, gold_rate, silver_rate, source, generated_at)
VALUES ('$CITY', $GOLD, $SILVER, 'manual_override', NOW())
ON CONFLICT (city_id) DO UPDATE
  SET gold_rate     = EXCLUDED.gold_rate,
      silver_rate   = EXCLUDED.silver_rate,
      source        = EXCLUDED.source,
      generated_at  = EXCLUDED.generated_at;
"

# Re-warm the cache so Redis picks up the override
bash scripts/warmup_cache.sh
```

> Invoices generated against a `manual_override` rate display: *"Invoice uses a manually set rate — verify before sharing."*

### Step 5 — Recovery

Once Gemini is reachable again:

1. Verify the next scheduler run succeeds:
   ```bash
   docker logs mahaswarna_pricing --tail=50 | grep "snapshot ready"
   ```
2. Lift kill switches if activated:
   ```bash
   docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
     "UPDATE feature_flags SET value = false WHERE key = 'kill_switch_ws';"
   ```
3. Run `bash scripts/smoke_test.sh`
4. Document the outage in `docs/incidents/`

---

## Runbook: Pricing Service Failure

**Symptoms:** Circuit breaker open on the gateway; rates returning `503` or `_degraded: true` in BFF responses; WS connections dropping.

```bash
# 1. Check container state
docker ps | grep pricing
docker logs mahaswarna_pricing --tail=100

# 2. Verify fallback is active
curl -sf http://localhost:4000/v1/rates/mumbai | jq .stale
# Should return true (serving stale cache), not an error

# 3. Restart pricing (sends close frames to all WS clients before exit)
docker compose restart pricing

# 4. Wait for /health/ready (pricing calls core internal API with backoff — up to ~4 min)
timeout 300 bash -c 'until curl -sf http://localhost:4002/health/ready; do sleep 5; done'

# 5. Warm the cache
bash scripts/warmup_cache.sh

# 6. Smoke test
bash scripts/smoke_test.sh

# 7. Post timeline to #incident-sev2
```

---

## Runbook: Redis Failure

**Symptoms:** Redis connection errors across all services; rate cache misses; feature flags stale; WS fanout failing.

> 🔴 **SEV-1 SECURITY RISK:** JTI revocation fails open during a Redis outage. A user who has logged out can reuse their access token for up to 15 minutes. Act on Step 1 immediately.

### Step 1 — Contain (Time-Critical)

```bash
# Activate kill switches immediately to prevent WS connections and payment flows
# while JTI revocation is unavailable
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  UPDATE feature_flags
  SET value = true
  WHERE key IN ('kill_switch_ws', 'kill_switch_payments');
"

# Raise FREE-tier BFF rate limit before kill-switch polling load hits
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "UPDATE feature_flags SET value = 60 WHERE key = 'rate_limit_bff_free_rpm';"
```

### Step 2 — Diagnose

```bash
# Check container state
docker ps | grep redis
docker logs mahaswarna_redis-primary --tail=100

# Check Sentinel status
redis-cli -h redis-primary -p 6379 INFO replication
redis-cli -p 26379 SENTINEL masters

# Check memory (may have been OOM-killed)
docker stats mahaswarna_redis-primary --no-stream
```

### Step 3 — Restore

```bash
# If OOM: set maxmemory before restarting
redis-cli -h redis-primary CONFIG SET maxmemory 2gb
redis-cli -h redis-primary CONFIG SET maxmemory-policy allkeys-lru

# Restart
docker compose restart redis-primary
docker compose restart redis-replica redis-sentinel

# Re-warm the rate cache
bash scripts/warmup_cache.sh
```

### Step 4 — Verify and Lift Kill Switches

```bash
# Verify JTI revocation is functional BEFORE lifting kill switches
bash scripts/smoke_test.sh
# smoke_test.sh includes: logout test token → confirm 401 on reuse

# Only after smoke_test.sh passes:
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  UPDATE feature_flags
  SET value = false
  WHERE key IN ('kill_switch_ws', 'kill_switch_payments');
"
```

### Step 5 — Post-Incident

Any logout issued during the Redis outage window is unenforceable for up to 15 minutes post-restore. Query `audit_log` for rows where `action = 'logout'` and `occurred_at` falls within the outage window, and flag those user IDs for security review.

---

## Runbook: Database Issues

### Connection Pool Exhaustion

**Symptoms:** Slow queries; `connection pool exhausted` errors in service logs.

```bash
# Check active connections per service
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  SELECT application_name, state, COUNT(*)
  FROM pg_stat_activity
  GROUP BY application_name, state
  ORDER BY COUNT(*) DESC;
"

# Check for long-running queries
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  SELECT pid, now() - pg_stat_activity.query_start AS duration, query, state
  FROM pg_stat_activity
  WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
"

# Terminate stuck queries if necessary
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle in transaction';"
```

### Migration Procedure

```bash
# Dry-run first (pre_deploy_check.sh calls this automatically)
bash scripts/migrate.sh --service=core --dry-run

# Apply migration (run during a low-traffic window)
bash scripts/migrate.sh --service=core

# Rollback a single step if needed
migrate -path migrations/core -database $DATABASE_URL down 1

# Verify
bash scripts/smoke_test.sh
```

**Zero-downtime migration rules:**

- **Adding a column:** `ADD COLUMN new_col TYPE DEFAULT NULL` in step 1; add constraints in a later release
- **Removing a column:** stop reading/writing first; drop the column in a later release
- **Renaming a column:** add new + copy data + drop old across 3 separate deployments

### Backup and Restore

```bash
# Manual backup (scheduled backup also runs automatically via cron)
bash scripts/backup_postgres.sh

# Restore from backup
bash scripts/restore_postgres.sh <backup-filename>
# restore_postgres.sh: download → KMS decrypt → pg_restore → smoke test
```

> **RTO caveat:** The 30-minute RTO is aspirational. Actual RTO depends on DB size at restore time. Measure actual restore time in a monthly drill against a production-sized backup and update this figure. Until a drill has been completed, treat RTO as **UNKNOWN**.

### Data Cleanup

```bash
# Purge expired data (flag_audit > 1 year, expired sessions)
bash scripts/cleanup_old_data.sh

# Verify no overdue pending deletes remain after hard_delete_job run
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  SELECT COUNT(*) FROM users
  WHERE deleted_at IS NOT NULL
    AND deleted_at < NOW() - INTERVAL '30 days'
    AND hard_deleted_at IS NULL;
"
# Expect 0. If > 0: investigate hard_delete_job failure in Sentry.
```

---

## Runbook: WebSocket Disruption

**Symptoms:** Mass WS disconnects; clients showing stale banner; reconnect storms visible in pricing logs.

```bash
# 1. Check active WS connection count
curl -sf http://localhost:4002/metrics | grep ws_connections_active

# 2. Check for abnormal disconnect patterns in logs
docker logs mahaswarna_pricing --tail=200 | grep -i "close\|disconnect\|error"

# 3. If a reconnect storm is in progress — check the WS upgrade rate limit
#    ws_server.go enforces 20 upgrades/IP/min; if the storm is internal, increase temporarily

# 4. Graceful restart (sends close frames — prevents a reconnect storm)
docker compose restart pricing
#    stop_grace_period: 20s in docker-compose.prod.yml allows close frames to be sent

# 5. Verify WS is accepting connections
# NOTE: port :4002 is required — ws.mahaswarna.com is directly exposed per ADR-002 (no TLS proxy on 443)
wscat -c wss://ws.mahaswarna.com:4002 -H "Authorization: Bearer $TEST_JWT"

# 6. Warm the cache
bash scripts/warmup_cache.sh
```

---

## Runbook: Play Billing Issues

**Symptoms:** Users reporting subscriptions not activating after payment; `receipt_log` rows stuck in `pending` state.

```bash
# 1. Check for pending receipts older than 5 minutes
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  SELECT id, user_id, status, created_at
  FROM receipt_log
  WHERE status = 'pending' AND created_at < NOW() - INTERVAL '5 minutes'
  ORDER BY created_at DESC LIMIT 20;
"

# 2. Check Google Play Developer API reachability
curl -sf "https://androidpublisher.googleapis.com/androidpublisher/v3/applications" \
  --header "Authorization: Bearer $(cat $GOOGLE_SERVICE_ACCOUNT_JSON | jq -r .access_token)" | jq .

# 3. Check verify_receipt_usecase logs in Sentry for the affected user_id

# 4. Manual receipt re-verification (if a user is stuck)
#    Ask the user to tap "Restore Purchase" in the app.
#    This calls POST /billing/restore, which re-runs verification against Google Play
#    and updates the subscription record.

# 5. If Google Play API is down — pause payment flows via feature flag
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "UPDATE feature_flags SET value = true WHERE key = 'kill_switch_payments';"
```

---

## Secret Rotation

### `INTERNAL_JWT_SECRET` Rotation (Zero-Downtime)

```bash
bash scripts/rotate_secrets.sh
```

The script implements a two-phase rotation:

1. New secret generated and deployed alongside the old (`NEW_INTERNAL_JWT_SECRET`)
2. Grace window: all services accept HMAC tokens signed with either secret
3. Old secret removed after all in-flight requests complete

### RS256 JWT Key Rotation (Zero-Downtime)

> Rotating JWT keys without a maintenance window requires multi-key acceptance. Do not rotate in a single step — any in-flight access token signed with the old private key will return `401` for up to 15 minutes.

```bash
# Step 1 — Generate a new key pair
openssl genrsa -out jwt_new.key 2048
openssl rsa -in jwt_new.key -pubout -out jwt_new.pub

# Step 2 — Deploy new PUBLIC key alongside the existing key (both accepted)
#           Add NEW_JWT_PUBLIC_KEY to .env.production
#           gateway jwt_pre_validator.go and core jwt_auth.go must accept EITHER key
#           (implemented as []rsa.PublicKey tried in order)

# Step 3 — Wait for the access token TTL to expire (15 minutes)
#           All tokens signed with the old private key are now expired

# Step 4 — Rotate JWT_PRIVATE_KEY to the new key; redeploy all services

# Step 5 — Remove the old public key from the NEW_JWT_PUBLIC_KEY fallback
```

Key storage: both keys in `.env.production` (encrypted at rest).

**Rotation schedule:** Immediately on suspected compromise; annually for routine rotation.

### `GEMINI_API_KEY` Rotation

```bash
# 1. Generate a new key in Google Cloud Console
# 2. Update GEMINI_API_KEY in .env.production
# 3. Restart both consumers of GEMINI_API_KEY:
#      pricing     — Gemini AI rate generation (ai_rate_scheduler_job.go)
#      intelligence — Gemini Vision content moderation (moderation_client.go, banner upload)
docker compose restart pricing intelligence
# 4. Verify rates are still updating and banner moderation is healthy
bash scripts/smoke_test.sh
```

### Firebase Service Account Rotation

```bash
# 1. Generate a new service account JSON in Firebase Console
# 2. Update FIREBASE_SERVICE_ACCOUNT_JSON in .env.production
# 3. Restart core service only (the only consumer)
docker compose restart core
# 4. Verify OTP flow works
bash scripts/smoke_test.sh
```

---

## Database Operations

### Adding a New City

```bash
# Insert into the cities table
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c "
  INSERT INTO pricing.cities (id, name, state, display_name)
  VALUES ('new-city-id', 'New City', 'State Name', 'New City');
"

# The next ai_rate_scheduler_job run includes it automatically

# Manually warm the cache for the new city immediately
SERVICE_TOKEN=$(bash scripts/gen_service_token.sh)
curl -sf "http://pricing:4002/rates/new-city-id" \
  -H "X-Service-Token: $SERVICE_TOKEN" > /dev/null
```

### Feature Flag Management

```bash
# View all flags
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "SELECT key, value, updated_at FROM feature_flags ORDER BY key;"

# Toggle a boolean flag
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "UPDATE feature_flags SET value = true, updated_at = NOW() WHERE key = 'kill_switch_ws';"

# Update a numeric parameter (e.g., sanity threshold)
docker exec -it mahaswarna_postgres psql -U $POSTGRES_USER -c \
  "UPDATE feature_flags SET value = 3.0, updated_at = NOW() WHERE key = 'rate_sanity_threshold_pct';"

# Gateway picks up changes via Redis pub/sub within ~60s — no restart needed
```

### SQLite VACUUM (Android Diary)

For users with large Diary databases (high-volume billing, extended use), schedule a periodic VACUUM after large delete operations:

```kotlin
// Run in a background coroutine — never on the main thread
appDatabase.openHelper.writableDatabase.execSQL("VACUUM")
```

---

## Scaling Triggers

Monitor these metrics in Grafana and act when thresholds are crossed:

| Metric | Threshold | Action |
|---|---|---|
| `pg_stat_activity` wait events (lock/IO) | > 5% of queries | Add PG read replica (already designed in architecture) |
| Redis memory usage | > 60% of `maxmemory` | Increase to 4 GB or move to managed Redis |
| Pricing service CPU | > 70% sustained | Extract pricing to a dedicated VPS (K8s prep) |
| WS concurrent connections | > 5,000 | Shard `connection_registry.go`; add a second pricing node |
| DAU | > 50,000 | Begin K8s migration — extract `pricing` + WebSocket first |
| BFF p95 latency | > 500ms consistently | Profile `home_aggregator.go` for upstream bottlenecks |
| Gateway 5xx rate | > 0.5% for 2 min | SEV-1 — immediate investigation |

> **SLO vs alert threshold:** The SLO target is < 0.1% 5xx error rate; the alert fires at > 0.5%. The gap is intentional — the 0.5% threshold filters transient spikes (e.g., a momentary upstream hiccup) while the SLO tracks sustained quality. If the error rate rises above 0.1% but stays below 0.5%, it appears in Grafana SLO dashboards as an SLO burn event requiring investigation within the next business hour (treat as SEV-3). Consider adding a SEV-2 alert at 0.2% sustained for 5 minutes if SLO burn becomes a recurring issue.

### Current HA Configuration (Option A — Launch)

```
Primary VPS (CPX41):   all 4 services
Standby VPS (CX22):    gateway + pricing only (warm standby)
Failover:              manual DNS switch (TTL = 60s) → RTO ~5 minutes
```

Redis Sentinel is required regardless of which application HA option is chosen.

### Recommended HA Path (Option B — Post-Launch)

```
Hetzner Load Balancer → CPX31 #1 (active, all 4 services)
                      → CPX31 #2 (active, scale-out)
Dedicated DB node:      CPX41 (PostgreSQL + Redis Sentinel)
RTO:                    zero (both nodes active)
Cost:                   ~$30/month additional
```

---

## Environment Variables Reference

All required variables are validated by `env_config_check.sh` at startup. Missing or malformed variables cause `os.Exit(1)`.

| Variable | Service(s) | Description |
|---|---|---|
| `APP_ENV` | All | `development \| staging \| production` |
| `DATABASE_URL` | All | PostgreSQL connection string (pgx DSN) |
| `REDIS_SENTINEL_1` | All | Redis Sentinel node 1 address (e.g. `redis-sentinel-1:26379`) |
| `REDIS_SENTINEL_2` | All | Redis Sentinel node 2 address |
| `REDIS_SENTINEL_3` | All | Redis Sentinel node 3 (tie-breaker) |
| `JWT_PRIVATE_KEY` | `core` | RS256 signing key (PEM) |
| `JWT_PUBLIC_KEY` | `gateway`, `core` | RS256 verification key (PEM) |
| `INTERNAL_JWT_SECRET` | All | Service-to-service HMAC-SHA256 signing (≥ 64 chars) |
| `OTP_PROVIDER` | `core` | `firebase \| msg91 \| both` |
| `FIREBASE_PROJECT_ID` | `core` | Required when `OTP_PROVIDER != msg91` |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | `core` | Firebase Admin SDK credentials |
| `MSG91_AUTH_KEY` | `core` | MSG91 API auth key |
| `MSG91_TEMPLATE_ID` | `core` | DLT-registered OTP template ID |
| `MSG91_OTP_EXPIRY_MINUTES` | `core` | OTP TTL (default: `10`; warn but do not exit if absent) |
| `GOOGLE_PLAY_PACKAGE_NAME` | `core` | IAP validation (Android only) |
| `GOOGLE_SERVICE_ACCOUNT_JSON` | `core` | Play Developer API auth |
| `PLAY_INTEGRITY_DECRYPTION_KEY` | `core` | Play Integrity token verification |
| `GEMINI_API_KEY` | `pricing`, `intelligence` | Gemini AI — server only; never forwarded to clients |
| `S3_BUCKET` | `intelligence` | Shop banner and DB backup storage |
| `S3_ENDPOINT` | `intelligence` | S3-compatible endpoint (AWS S3 or Hetzner Object Storage) |
| `KMS_KEY_ARN` | `intelligence` | AWS KMS key for backup encryption — required when using AWS S3 only |
| `CDN_BASE_URL` | `intelligence` | CDN base URL (must be HTTPS in production) |
| `SENTRY_DSN` | All | Error tracking |
| `PAGERDUTY_KEY` | Alertmanager | Alertmanager → PagerDuty routing |
| `PROMETHEUS_REMOTE_WRITE_URL` | Prometheus | Metrics export endpoint |

> **iOS variables** (`APPLE_BUNDLE_ID`, `APPLE_SHARED_SECRET`) are not included — MahaSwarna is Android-only. Add them alongside `app_store_client.go` when iOS support is added.
