# MahaSwarna — Security

> **Authority:** This is the authoritative reference for all security-sensitive decisions across the Go backend and Kotlin Android client. It covers the threat model, authentication controls, token lifecycle, device integrity, network security, secret management, data protection, compliance requirements, known accepted risks, and incident response.

---

## Table of Contents

- [Threat Model Summary](#threat-model-summary)
- [Authentication](#authentication)
- [Token Lifecycle](#token-lifecycle)
- [Device Integrity](#device-integrity)
- [Network Security](#network-security)
- [Secret Management](#secret-management)
- [Android Security Controls](#android-security-controls)
- [Data Protection](#data-protection)
- [Compliance Requirements](#compliance-requirements)
- [Known Accepted Risks](#known-accepted-risks)
- [Incident Response](#incident-response)

---

## Threat Model Summary

| Threat | Control |
|---|---|
| Credential theft via log exfiltration | JWT, receipt tokens, and API keys are never written to any log at any level; `LogRedactionInterceptor` strips `Authorization` + `Set-Cookie` |
| Token replay after logout | JTI revocation list in Redis; 401 cascades to a forced logout |
| Rooted/emulated device bypassing the purchase funnel | Play Integrity attestation on login **and** before any purchase endpoint |
| Man-in-the-middle attack | TLS + intermediate CA public key pinning on Android (primary + backup pin) |
| Client-supplied purchase status fraud | DB subscription record is the sole source of truth — no endpoint trusts client-provided tier |
| Rate abuse / scraping | Redis token-bucket rate limiter per tier (FREE/PREMIUM/ADMIN); abuse detector with 1-hour IP block |
| Service-to-service impersonation | HMAC-SHA256 `X-Service-Token` on every internal call; all services on the same Docker bridge network |
| Secret leak | `.env.production` encrypted at rest; Sentry/Grafana log redaction; rotated via `rotate_secrets.sh` |
| WS DoS via upgrade storm | WS handshake rate limit (20 new upgrades/IP/min via Redis) enforced in `ws_server.go` |
| Invoice rate fraud | Rate source resolution priority: **client_override → manual_override → stale → live** (client_override is highest; live is the default fallback). Any source other than `"live"` surfaces a client-visible warning. See `ARCHITECTURE.md — Intelligence` for the full priority table. |
| API replay attacks | Idempotency layer caches POST responses by `Idempotency-Key` (Redis 24h TTL) |
| OTP brute-force | Max 5 OTP sends/phone/hour; max 10 failed login attempts/phone/15 min |

---

## Authentication

### OTP — Dual-Provider Architecture

| Provider | Verification Flow | When Used |
|---|---|---|
| Firebase Phone Auth (primary) | Client-side SMS trigger → Firebase ID token → server verifies via Admin SDK | Default (`OTP_PROVIDER = firebase` or `both`) |
| MSG91 SMS (fallback) | Backend sends OTP via MSG91 v5 REST → client submits 6-digit code → backend verifies | `OTP_PROVIDER = msg91` or Firebase failure in `both` mode |

**Phone normalisation (required before any provider call):**

All phone numbers are stored and compared in E.164 format: `+91XXXXXXXXXX`. Strip leading `0`, `+91`, `91`, spaces, and hyphens before normalising. Redis rate-limit keys use E.164 — un-normalised keys create separate counters for the same number, silently bypassing the limit.

**Firebase verification sequence:**

```
1. Play Integrity token verified (required — see Device Integrity)
2. Backend calls firebaseAuthClient.VerifyIDToken(ctx, idToken)
3. Extract phone_number claim; verify it matches the request's phone field (E.164)
4. On success:  upsert user → issue JWT pair
5. On failure:  HTTP 401 { "error": "otp_invalid" }
```

**MSG91 verification sequence:**

```
1. Play Integrity token verified
2. GET api.msg91.com/api/v5/otp/verify (authkey + mobile + otp)
3. On 200 + type == "success": verified → upsert user → issue JWT pair
4. On failure:  HTTP 401 { "error": "otp_invalid" }
```

**OTP rate limits:**

- Max 5 OTP sends per phone per hour — Redis key `otp_send:{E164phone}`, INCR + EXPIRE 3600s
- Max 10 failed login attempts per phone per 15 minutes — Redis key `login_fail:{phone}`
- Every send and verify attempt (success or failure) is written to `audit_log` with `actor=phone`, `action=otp_send|otp_verify`, `metadata={provider, success, ip}`

### Service-to-Service Authentication

All internal calls carry `X-Service-Token: HMAC-SHA256(requestTimestamp + INTERNAL_JWT_SECRET)`. All four services share one secret via the `INTERNAL_JWT_SECRET` env var.

- `INTERNAL_JWT_SECRET` must be ≥ 64 characters — validated in `env_config_check.sh`
- A missing secret causes a hard startup crash (`os.Exit(1)`) — a missing secret is a silent auth bypass
- Token generation and verification are consolidated in `src/shared/service_token.go`
- `@Named("s3")` OkHttpClient (Android) must **not** include `AuthInterceptor` — presigned S3 URLs reject the `Authorization` header

---

## Token Lifecycle

### JWT Claims

| Claim | Type | Value |
|---|---|---|
| `sub` | string | User ID (UUID) |
| `jti` | string | Unique token ID (used for revocation) |
| `tier` | string | `FREE \| PREMIUM \| ADMIN` |
| `region` | string | City ID set at registration (e.g. `mumbai`) |
| `iat` | int | Issued-at timestamp |
| `exp` | int | Expiry timestamp |

- **Access token TTL:** 15 minutes
- **Refresh token TTL:** 30 days (stored in DB, revocable by JTI)
- `region` is embedded at login — changes to a user's city require a token refresh and take effect after up to 15 minutes

### Android Token Storage

```
EncryptedSharedPreferences (AES-256-GCM, Android Keystore)
  └── access_token    ← never plain SharedPreferences
  └── refresh_token

token_exists_marker (plain file in filesDir)
  └── written AFTER token is committed to EncryptedSharedPreferences
  └── deleted on logout / clearSessionData()
  └── used by SplashScreen for routing — zero Keystore access, zero TEE latency
```

**Write order invariant (`TokenStore.saveAccessToken`):**

1. `prefs.edit().putString("access_token", token).commit()` — use `commit()`, not `apply()` (async)
2. `File(filesDir, "token_exists_marker").createNewFile()`

> **Order matters:** If the process is killed between steps 1 and 2, the token exists but the marker is absent → SplashScreen routes to Login (safe). The reversed order risks routing to Home with no token → 401 cascade.

### JTI Revocation

JTIs are added to a Redis set on logout or token refresh. Every JWT validation (`gateway/jwt_pre_validator.go`) checks the revocation set before accepting a token.

> **Redis outage risk:** If Redis is unreachable, JTI revocation fails open — a logged-out user can reuse their access token for up to 15 minutes (the access token TTL). Mitigation: activate `kill_switch_ws` and `kill_switch_payments` immediately on Redis failure (see [`RUNBOOK.md`](RUNBOOK.md)). After Redis is restored, verify revocation is functional via `smoke_test.sh` before lifting kill switches.

### Zero-Downtime JWT Key Rotation

RS256 keys are rotated without downtime using multi-key acceptance:

```go
// jwt_pre_validator.go maintains a slice of accepted public keys
keys := []rsa.PublicKey{primaryKey}
if newKey != nil { keys = append(keys, newKey) }
// Try each key in sequence; accept on first successful verification
```

Full rotation procedure: see [`RUNBOOK.md — Secret Rotation`](RUNBOOK.md#secret-rotation).

---

## Device Integrity

### Play Integrity — Required on Login

`POST /auth/login` must include `integrityToken` in the request body. The backend verifies the token via the Google Play Integrity API before issuing any JWT.

**On integrity failure:** `HTTP 403 { "error": "device_not_trusted" }`. The Android client shows a non-dismissible "This device is not supported" screen and does not navigate to Home.

**Rationale:** A rooted or emulated device that passes OTP verification receives a valid JWT and can consume live rates indefinitely on the FREE tier — bypassing the purchase funnel entirely. The pre-purchase Play Integrity check is retained as a second enforcement layer.

**Android (`LoginViewModel`):** Must obtain a Play Integrity token via `IntegrityManager.requestIntegrityToken()` before calling `AuthRepository.login()`. On `IntegrityManager` failure (Play Services unavailable), surface this as a login error — do not silently proceed without a token.

### Play Integrity — Pre-Purchase

Play Integrity is called before any billing endpoint executes. This is enforced server-side in `billing_handler.go` and cannot be bypassed by the client.

---

## Network Security

### TLS and Certificate Pinning (Android)

Pin the **intermediate CA public key**, not the leaf certificate. Leaf pinning breaks every 90 days with Let's Encrypt and is a self-inflicted outage.

```kotlin
// RetrofitClient.kt
CertificatePinner.Builder()
    .add("api.mahaswarna.com", "sha256/<primary_intermediate_ca_pin>")
    .add("api.mahaswarna.com", "sha256/<backup_pin>")    // next CA or pre-generated key
    .build()
// Same pinning applies to ws.mahaswarna.com (WSS)
```

**Pin rotation procedure:**

1. Ship a new release with `backup_pin` added alongside `primary`
2. Wait for > 90% of users on the new release (Play Console)
3. Rotate the server certificate or CA
4. Ship a release promoting `backup_pin` to `primary`

### Network Security Config (Android Debug)

`res/xml/network_security_config.xml` permits cleartext HTTP to `10.0.2.2` and `localhost` for debug builds only. TLS is enforced in all release builds. Referenced in `AndroidManifest.xml` via `android:networkSecurityConfig`.

### WebSocket — Production TLS Enforcement

In production, `ws_server.go` rejects plain HTTP upgrade requests:

```go
if os.Getenv("APP_ENV") == "production" && r.TLS == nil {
    http.Error(w, "WSS required", http.StatusForbidden)
    return
}
```

`smoke_test.sh` asserts that plain HTTP to `:4002` returns `403`.

### Hetzner Firewall Rules

| Port | Protocol | Allowed From | Purpose |
|---|---|---|---|
| 443 | TCP | `0.0.0.0/0` | HTTPS via reverse proxy |
| 4000 | TCP | `0.0.0.0/0` | API gateway REST |
| 4002 | TCP | `0.0.0.0/0` | WebSocket (WSS only in production) |
| 22 | TCP | Ops IP range only | SSH access |

> Ports 4001 (`core`) and 4003 (`intelligence`) are **not** exposed to the public internet — Docker bridge network only.

### Abuse Detection

`gateway/middleware/abuse_detector.go` implements heuristic abuse detection:

- Request burst: > 100 req/10s from the same IP → `HTTP 429`
- Payload probing: repeated 4xx pattern → IP blocked for 1 hour
- All signals written to Redis sorted sets per IP
- `http.MaxBytesReader(w, r.Body, 64*1024)` applied in the router middleware chain
- Global 3s context deadline on all gateway requests

---

## Secret Management

### Current Policy (Launch)

`.env.production` encrypted at rest via [`age-encryption`](https://age-encryption.org) or `git-crypt` with a hardware-backed master key. The key is stored offline — not on the VPS. The file is never committed to git.

### Post-Launch Upgrade (Target: 30 days)

Migrate to HashiCorp Vault OSS (self-hosted Docker container on the VPS):

- Per-service Vault policies (`core` cannot read `pricing` keys)
- Audit log of every secret read (Vault audit backend → Loki)
- Automatic rotation for `INTERNAL_JWT_SECRET` and DB passwords
- Each service reads secrets via `VAULT_TOKEN` at boot; renewal handled by vault-agent sidecar

### What Must Never Happen

- Secrets in git history (in any branch, including squashed commits)
- Secrets in Docker image layers (`docker history` reveals env vars baked into images)
- Secrets in `docker-compose.yml` container env vars — use the `secrets:` block
- Secrets in Sentry payloads, Grafana dashboards, or structured log entries
- API keys in any Android APK/AAB (the Gemini API key is server-only; clients receive data, not keys)

### Leak Response

If any secret is suspected compromised:

1. Rotate immediately via `bash scripts/rotate_secrets.sh`
2. Revoke all active JTIs: `UPDATE sessions SET revoked = true`
3. Notify affected users if `JWT_PRIVATE_KEY` was exposed (all issued tokens are compromised)
4. Write an incident report in `docs/incidents/`

---

## Android Security Controls

| Control | Implementation |
|---|---|
| Token storage | EncryptedSharedPreferences (AES-256-GCM, Android Keystore) |
| Token routing | Lazy Keystore access via `AuthInterceptor` — never in `Application.onCreate()` |
| Log redaction | `LogRedactionInterceptor` strips `Authorization` + `Set-Cookie` from OkHttp logs |
| Crash payloads | No PII in Crashlytics payloads; API errors logged with `X-Trace-ID` for correlation only |
| Paywall screen | `FLAG_SECURE` set via `DisposableEffect`; `clearFlags` called in `onDispose` — failing to clear leaves the flag active on all subsequent screens |
| Device integrity | Play Integrity API on login and before any purchase flow |
| Diary data | Local Room only — never transmitted to any backend endpoint |
| Network | TLS + intermediate CA pinning with backup pin; `LogRedactionInterceptor` strips sensitive headers |
| Notifications | `POST_NOTIFICATIONS` runtime request guarded with `Build.VERSION.SDK_INT >= TIRAMISU` |
| Camera permission | Runtime request with rationale; graceful denial → gallery-only fallback |
| Deprecated API | `VersionInterceptor` → `ApiError.VersionDeprecated` → non-dismissible `UpdateRequiredScreen` on HTTP 410 |

---

## Data Protection

### Data Classification

| Data | Classification | Storage | Transmitted |
|---|---|---|---|
| Access token / refresh token | Highly sensitive | EncryptedSharedPreferences only | HTTPS only; never logged |
| Phone number | Sensitive PII | PostgreSQL (hashed for Redis keys) | HTTPS only |
| Diary bills, ledger, customers | Sensitive PII | Local Room only | **Never** |
| Gold/silver rates | Non-sensitive | PostgreSQL + Redis | HTTPS + WSS |
| Feature flags | Non-sensitive | Redis + local DataStore | HTTPS |
| FCM push token | Device identifier | PostgreSQL (per userID + deviceID) | HTTPS |
| Invoice PDF | Financial document | Local device storage only | **Never stored server-side** |
| Shop banner | Public | S3-compatible storage | CDN (HTTPS) |

### Consent

Consent must be obtained and logged before any data is processed:

- Privacy Policy, Terms of Service, and AI Disclaimer are shown on first launch (`Route.Consent`)
- `POST /user/consent` is idempotent — safe to re-call on reinstall
- The consent table is insert-only: `REVOKE UPDATE, DELETE ON consent_log FROM app_role`
- `PreferenceStore.consentAccepted` gates all subsequent SplashScreen routing

### Data Retention

| Data | Retention Period | Mechanism |
|---|---|---|
| `ai_rate_snapshots` | 30 days | Deleted by `cleanup_old_data.sh` |
| `flag_audit` rows | 1 year | Deleted by `cleanup_old_data.sh` |
| Soft-deleted users | 30-day grace period | Hard DELETE by `hard_delete_job.go` |
| `audit_log` (non-billing) | 2 years | Archived to S3, then deleted |
| `audit_log` (billing) | 7 years | Retained — financial compliance |
| `receipt_log` | 7 years | Retained — financial compliance |
| `consent_log` | Indefinitely | Retained — legal evidence |

### Account Deletion

`DELETE /user/account` flow:

1. Extract `userID` from the JWT `sub` claim (no `:userID` path parameter)
2. Soft-delete the user row (`deleted_at = NOW()`)
3. Revoke all active JTIs
4. `pg NOTIFY account_deleted` → intelligence purges shops and invoices
5. Schedule hard-delete after the 30-day grace period
6. Write an audit entry
7. Return `204 No Content`

Hard-delete is executed by `hard_delete_job.go` (daily cron). Cross-schema cascade is handled exclusively via the `account_deleted` pg NOTIFY listener in `intelligence/events/listeners.go`.

---

## Compliance Requirements

### TRAI DLT Compliance (Indian SMS)

MSG91 is used as an OTP fallback. All OTP SMS messages must be sent via TRAI-registered DLT templates. `MSG91_TEMPLATE_ID` must reference a DLT-approved template. Non-compliant messages will be blocked by Indian telecom operators.

### Append-Only Tables

The following tables have `REVOKE UPDATE, DELETE` grants to prevent tampering:

| Table | Service | Purpose |
|---|---|---|
| `consent_log` | core | Legal evidence of user consent |
| `receipt_log` | core | Financial audit trail |
| `audit_log` | core | Immutable action log |

### Audit Log Requirements

Every security-relevant action must write to `audit_log` with the following fields:

- **Actor:** `userID` or `"system"`
- **Action:** e.g. `otp_send`, `otp_verify`, `login`, `logout`, `delete_account`, `hard_delete`
- **Entity and entity ID**
- **Metadata:** JSONB (no secrets)
- **`occurred_at`:** timestamptz

### Dual-Fire Coverage (Compliance Invariant)

`delete_account_usecase_test.go` must cover all three test cases for `account_deleted`:

- **Test A:** User-initiated deletion — assert NOTIFY fires once; audit entry written with `actor=userID`
- **Test B:** System-initiated hard-delete — assert NOTIFY fires once; `hard_deleted_at` set; audit entry written with `actor="system"`
- **Test C:** Double-fire idempotency — two fires for the same `userID`; intelligence DELETE is idempotent

CI fails if this file is absent (`test -f test/core/delete_account_usecase_test.go || exit 1`).

---

## Known Accepted Risks

### Redis Outage — JTI Revocation Fails Open

**Risk:** If Redis is unreachable, a logged-out user can reuse their access token for up to 15 minutes (the access token TTL). JTI revocation is unavailable during the outage window.

**Accepted:** Yes — a distributed token blacklist without Redis would require a database read on every request, eliminating the performance advantage of Redis-cached JWT validation.

**Mitigations:**

- Redis Sentinel (3-node) reduces the probability of a Redis outage to hardware failure of the primary node during the Sentinel failover window (~10–30s)
- Access token TTL is 15 minutes — the maximum exposure window is bounded
- `kill_switch_ws` and `kill_switch_payments` must be activated immediately on Redis failure (see [`RUNBOOK.md`](RUNBOOK.md))
- Any logout issued during a Redis outage window must be flagged for security review post-incident

### WebSocket Gateway Bypass

**Risk:** Port `:4002` bypasses gateway middleware (rate limiting, circuit breakers, abuse detection, JWT pre-validation).

**Accepted:** Yes — see ADR-002 in [`ARCHITECTURE.md`](ARCHITECTURE.md). Compensating controls are implemented in `ws_server.go`.

### Single VPS — No Application-Layer Redundancy

**Risk:** A single VPS restart takes all 4 services offline simultaneously. RTO is ~5 minutes (DNS TTL = 60s + manual failover to warm standby).

**Accepted:** Yes, at launch. Redis Sentinel HA is mandatory; application-layer HA via Hetzner Load Balancer + 2× CPX31 is the recommended post-launch upgrade path.

---

## Incident Response

### Severity Levels

| Severity | Definition | Response SLA | Resolution Target |
|---|---|---|---|
| 🔴 **SEV-1** | Complete outage, data loss, or security breach | Immediate page 24/7; Eng Lead notified in 5 min; postmortem within 48h | 30 minutes (aspirational — see RTO caveat in `RUNBOOK.md`) |
| 🟡 **SEV-2** | Significant degradation affecting > 10% of users or revenue | Page; update `#prod-alerts`; status page updated every 30 min | 2 hours |
| 🟢 **SEV-3** | Minor degradation with no immediate user impact | Slack `#prod-warnings`; investigate within 4h | 24 hours |

### Security Breach Response

1. **Contain:** Rotate all affected secrets immediately via `bash scripts/rotate_secrets.sh`
2. **Revoke:** `UPDATE sessions SET revoked = true` — forces all users to re-authenticate
3. **Assess:** Identify scope from `audit_log` — what was accessed, by whom, and for how long
4. **Notify:** Inform affected users if `JWT_PRIVATE_KEY` was exposed (all issued tokens are compromised)
5. **Document:** Write an incident report in `docs/incidents/{date}-{title}.md`
6. **Post-mortem:** Required within 48h for SEV-1

### Emergency Contacts

Configure the following before go-live:

- `PAGERDUTY_KEY` for Alertmanager → PagerDuty routing (SEV-1 and SEV-2 alerts)
- `SENTRY_DSN` for error capture
- On-call rotation in PagerDuty covering at minimum the founding engineering team
