# MahaSwarna Frontend — Complete File Structure
> Native Android architecture (Kotlin). Aligned to the Go backend (4 services: gateway :4000, core :4001, pricing :4002, intelligence :4003). Optimised for WhatsApp-like cold start: meaningful UI rendered from local cache within 400ms, WebSocket connected within 1–2 seconds, no network call blocks the first frame. Features: Rates, Calculator, Catalog, ShopBanner, BillPrint, Diary. iOS is not supported.

---
## Table of Contents
- [Architecture Decision Records](#architecture-decision-records)
- [Tech Stack](#tech-stack)
- [Architecture Overview](#architecture-overview)
- [Cold Start Strategy](#cold-start-strategy)
- [Cross-Cutting Invariants](#cross-cutting-invariants)
- [Android — Root](#android--root)
- [Android — Core](#android--core)
- [Android — Features](#android--features)
- [Android — Data Layer](#android--data-layer)
- [Android — UI/Compose](#android--uicompose)
- [Shared Contracts](#shared-contracts)
- [Observability & Analytics](#observability--analytics)
- [Security](#security)
- [Compliance & Permissions](#compliance--permissions)
- [Release & CI/CD](#release--cicd)

---

## Architecture Decision Records

### ADR-001 — Invoice PDF Wire Format (DECIDED)

**Canonical location:** backend architecture doc.
**Summary:** Option A — JSON wrapper with base64-encoded PDF bytes. `InvoiceResponse.pdfBytes`
is a `ByteArray` decoded automatically by kotlinx.serialization. Retrofit return type is
`InvoiceResponse` (not `ResponseBody`). Do not reopen this decision.

Retrofit declaration:
```kotlin
@POST("shops/{id}/invoice/generate")
suspend fun generateInvoice(
 @Path("id") shopId: String,
 @Body request: GenerateInvoiceRequest
): InvoiceResponse
```

See backend architecture doc ADR-001 for full rationale and Go implementation.

---

## Tech Stack

| Concern | Android |
|---|---|
| Language | Kotlin **2.2.20** |
| Min Android SDK | **24** (Android 7.0 Nougat). Rationale: `EncryptedSharedPreferences` requires API 23+; `Play Integrity API` requires Google Play Services; `SplashScreen` compat lib supports API 21+ but API 24 covers >99% of the Indian Android market on budget devices (Redmi Note, Realme C-series). **`fallbackToDestructiveMigration()` is BANNED** — set this at Room DB builder creation time and it must never appear in `AppDatabase.kt`. |
| UI Framework | Jetpack Compose (Material3) |
| HTTP Client | Retrofit **3.0.0** + OkHttp **5.x** (`okhttp-android` artifact). Retrofit 3 includes built-in kotlinx.serialization support — no separate converter artifact needed.|
| WebSocket | OkHttp 5 `WebSocket` API (same `okhttp-android` artifact) |
| Auth Storage | EncryptedSharedPreferences (Jetpack Security) |
| OTP Auth | **Dual-provider:** Firebase Phone Authentication (primary) + MSG91 SMS OTP (fallback). Firebase flow is entirely client-side: `FirebaseAuth.getInstance().verifyPhoneNumber()` → receives `PhoneAuthCredential` + `firebaseIdToken` → sent to backend for server-side verification. MSG91 flow: backend sends OTP via MSG91; client receives SMS and submits the 6-digit code to backend via `POST /auth/login`. Provider is determined by the backend's `POST /auth/send-otp` response. |
| DI | Hilt |
| Async | Kotlin Coroutines + Flow |
| Local DB | Room **2.8.3** (SQLite) |
| Navigation | Compose Navigation |
| IAP | Google Play Billing Library 7 (`billing-ktx` — `-ktx` intentional; required for coroutine/suspend API; **explicit exception** to the Firebase `-ktx` ban ) |
| Push Notifications | FCM (Firebase Messaging) |
| Image Loading | Coil 3 |
| Charts | Vico (`com.patrykandpatrick.vico:compose-m3:2.x`) — used for gold/silver rate history line chart in `RateHistoryScreen.kt`. Must be compatible with the project's Compose BOM version; verify at https://github.com/patrykandpatrick/vico/releases. |
| PDF Generation | `android.graphics.pdf.PdfDocument` (platform API, API 19+) — Diary export only (local Room → PDF). iTextG is **NOT used** — it is distributed under AGPL, which requires open-sourcing the entire app or purchasing a commercial license. The platform API has zero license risk and is sufficient for structured receipt/table layouts at this scale. |
| Analytics | Firebase Analytics |
| Crash Reporting | Firebase Crashlytics |
| Feature Flags | In-app cache of `GET /config/feature-flags` |
| Linting | ktlint + Detekt |
| Testing | JUnit5 + Mockk + Turbine |
| CI | GitHub Actions |
| Firebase BOM | **34.0.0** — base artifacts only; **NO** `-ktx` suffix variants (BOM 34 bundles Kotlin extensions natively into base artifacts; using `-ktx` doubles the dependency and may cause duplicate class errors). Exception: `billing-ktx` — see IAP row. |

**Firebase `-ktx` ban:**
```
✅ firebase-analytics ❌ firebase-analytics-ktx
✅ firebase-crashlytics ❌ firebase-crashlytics-ktx
✅ firebase-messaging ❌ firebase-messaging-ktx
✅ firebase-perf ❌ firebase-perf-ktx
✅ firebase-auth ❌ firebase-auth-ktx
```

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────┐
│ MahaSwarna Android App │
│ Kotlin · Jetpack Compose · Hilt DI · Room (SQLite) │
│ │
│ Rates · Calculator · Catalog · ShopBanner · BillPrint · Diary │
└────────────────────────┬─────────────────────────────────────────┘
 │
 ┌──────────────────────▼───────────────────────────────────────┐
 │ Shared Contract Layer │
 │ DTOs, API constants, WS envelope types, error codes │
 └──────────────────────┬───────────────────────────────────────┘
 │ HTTPS / WSS
 ┌─────────▼─────────────────┐
 │ API Gateway :4000 │
 │ (BFF aggregation inline)│
 │ Realtime WS :4002 │
 └───────────────────────────┘
```

**Data Flow per Feature:**

```
UI (Compose)
 → ViewModel
 → UseCase (where domain logic justifies the layer)
 → Repository (interface)
 → LocalDataSource ←→ Room (shown immediately on launch)
 → RemoteDataSource ←→ Retrofit (refreshes in background)
 → WsDataSource ←→ OkHttp WS (live updates)

Calculator: pure local computation — no repository, no network
Diary: local-only — Room only, no RemoteDataSource, no network path
```

**Key principle:** Local DB is the source of truth on launch. Network is a background refresher. The UI never waits on a network call to render the first frame. Calculator and Diary are fully offline.

> **Use case layer discipline:** Use cases are added only where they encapsulate real domain logic (multi-step operations, data merging, complex validation). Simple CRUD operations (alert create/delete) are called directly from the ViewModel via the repository — a thin use case wrapper that delegates straight through adds ceremony without benefit.

---

## Cold Start Strategy

Target: meaningful UI rendered in **< 400ms**, WebSocket connected in **1–2 seconds**.

### Android — Exact Timing Budget

```
T+0ms User taps icon
T+0ms OS applies SplashScreen API instantly (zero Compose frames)
T+5ms Application.onCreate():
 - NotificationChannelSetup.createChannels() ← BEFORE Firebase
 - super.onCreate() → Hilt builds app component (NetworkModule, DatabaseModule, WsModule)
 - Room.openAsync() [non-blocking, background]
 - Firebase.initializeApp() [async, off critical path]
 - NOTE: TokenStore.init() (Keystore access) is NOT called here.
 On post-reboot cold start, first Keystore TEE/StrongBox access
 takes 50–200ms on budget devices (Realme C-series, Redmi Note).
 Calling it here would consume the entire 400ms budget margin.
T+5ms SplashScreen routing decision — uses token_exists_marker file
 (a plain file written on login, deleted on logout — zero Keystore access).
 if marker absent → navigate(Route.Login); return
 else → hold splash frame via OnPreDrawListener while DataStore
        consent read resolves asynchronously (lifecycleScope.launch):
        if !consentAccepted → navigate(Route.Consent); return
T+10ms MainActivity.onCreate()
T+10ms setContent {} → HomeScreen()
T+10–50ms RatesViewModel.init() kicks off:
 - ratesRepository.getCachedRates() [Room query, ~5–15ms]
T+50–80ms First Compose frame rendered from Room cache ← FIRST MEANINGFUL RENDER
 (stale banner shown if rate.isStale == true OR wsState != Connected for > 30s)
T+80ms Background coroutines launched (off main thread):
 - GET /config/feature-flags (from local cache — DEFAULT_FLAGS if first install)
 - GET /bff/home (background REST call)
 - JWT pre-warm (synchronous, on critical WS path):
 val remainingMs = sessionManager.accessTokenRemainingMs()
 if (remainingMs < 3 * 60_000L) {
 try { authRepository.refreshToken() }
 catch (e: Exception) { /* logged; WS will 401-retry */ }
 }
T+800ms WebSocket connect to wss://ws.mahaswarna.com:4002
 (token is now guaranteed valid for at least 12 min — no 401 on connect)
T+900ms WS JWT auth handshake
T+1000ms WS authenticated → subscribed to rates|alerts channels
T+1200ms GET /bff/home response arrives → Room updated (ALL fields persisted) → Flow emits
 → Compose recomposes with fresh data
T=DONE Live rates streaming via WebSocket
```

**First render target: 50–80ms ✅ (400ms budget has 5× headroom)**

**Post-reboot Keystore note:** TokenStore is accessed lazily by AuthInterceptor on the first REST call (T+80ms coroutine block), after the Compose frame has rendered. The 50–200ms TEE overhead on budget devices is absorbed entirely in the background — it does not block the first frame.

### Room as launch source

Android seeds the home screen from Room on every launch:

1. `RatesRepository` emits last cached rate instantly from Room.
2. In parallel, BFF REST call fetches fresh home data; on success, **all fields** (rates + alerts) are persisted to Room → Flow re-emits.
3. WebSocket takes over for live updates once connected.
4. **Stale indicator** is driven by the backend's `stale: Bool` field in `GetRateResponse` (`RateDto`) — **not** by a client-side `cachedAt > 15min` calculation. The `cachedAt` timestamp records when the *client* received data; the backend's `stale` field reflects whether the Gemini AI rate generator has produced fresh data within the IST market window. These are different signals. `isStale` must be derived from `rate.stale`. Additionally, if WebSocket has been disconnected for > 30 seconds, `StaleRateBanner` is shown regardless of `rate.stale`.

**First-install shimmer:** On fresh install, Room is empty — `LoadingShimmer` shows until BFF response arrives. The BFF target is < 1500ms (backend `warmup_cache.sh` ensures Redis is hot). **Shimmer must not persist beyond 2 seconds** — enforced by an explicit timeout in `HomeViewModel`, not by trusting BFF latency alone:

```kotlin
// MUST be the first launch in init() — before the Room cache collector is started.
// The Room collector calls shimmerJob?.cancel(); if shimmerJob is not yet assigned
// (because the shimmer launch comes second), the cancel is a no-op and the shimmer
// fires 2 seconds later, overwriting a valid Success state with NoDataAvailable.
shimmerJob = viewModelScope.launch {
 delay(2_000)
 if (_uiState.value is Loading) _uiState.value = NoDataAvailable
}
```

`NoDataAvailable` state shows a "No connection — tap to retry" UI. On no-network cold start (airplane mode), this ensures the shimmer resolves rather than spinning indefinitely.

---

## Cross-Cutting Invariants

- JWT access token (15min TTL) and refresh token (30 days) are **stored only in EncryptedSharedPreferences** — never in plain SharedPreferences.
- Tokens are **never logged** at any level. Log redaction applied in HTTP interceptors.
- All API errors are captured to Firebase Crashlytics with `X-Trace-ID` for backend correlation.
- Play Integrity attestation is performed before any purchase flow begins and on login.
- **Play Integrity on login (required):** `LoginViewModel` must obtain a Play Integrity token via `IntegrityManager.requestIntegrityToken()` before calling `AuthRepository.login()`. The token is included in the `POST /auth/login` request body alongside phone + OTP. On `HTTP 403 { "error": "device_not_trusted" }`: show a non-dismissible "This device is not supported" screen with a support link. Do NOT navigate to Home. On `IntegrityManager` failure (Play Services unavailable): surface as a login error — do not silently proceed without a token.
- Client **never trusts its own purchase state** — subscription tier is read exclusively from the JWT `tier` claim refreshed after a successful `/billing/verify` call.
- Feature flags are fetched on app resume and cached locally; kill-switches (`ai`, `ws`, `payments`, `catalog`) gate entire feature entry points. **On first install, `DEFAULT_FLAGS` are used until the first successful fetch.**
- WebSocket connects only after a valid JWT is confirmed; reconnects with exponential backoff (1s → 2s → 4s … 60s cap, inline in `WsClient`). **JWT is pre-warmed synchronously in the background coroutine (T+80ms) before WS connect.**
- **WS kill-switch fallback:** When `killSwitchWs == true`, `WsClient.connect()` is NOT called. The app operates in polling-only mode: `HomeViewModel` triggers a REST refresh via `GET /bff/home` every **30 seconds** while the screen is **foregrounded**. Implementation via `lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED)`, NOT a bare `viewModelScope.launch { while(true) { delay(30_000) } }` which keeps polling even when the user navigates away.

 **Kill-switch load warning:** In polling mode, 1,200 concurrent users generate ~40 RPS against `/bff/home` — matching the normal BFF peak ceiling. The loop MUST include ±5s jitter to prevent thundering herd on resume. Notify the backend team to raise the FREE-tier BFF rate limit before activating the kill-switch at scale.

 Canonical implementation (`HomeScreen.kt` — requires `import kotlin.random.Random`):
 ```kotlin
 // HomeScreen.kt LaunchedEffect
 lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {
 while (true) {
 delay(30_000L + Random.nextLong(-5_000L, 5_000L)) // ±5s jitter — mandatory
 homeRepo.refresh()
 }
 }
 ```
 `StaleRateBanner` is shown permanently in this state.
- **Stale rate indicator** is driven by the backend `rate.stale` field, not by client-side time calculations.
- **WS disconnect stale indicator:** If WebSocket transitions to `RECONNECTING` or `DISCONNECTED` for > 30 seconds, `StaleRateBanner` is shown regardless of `rate.stale`. If WebSocket transitions to `ERROR`, `StaleRateBanner` is shown immediately (no 30-second grace — `Error` is not a transient state).
- **First frame never blocked by network.** Every screen that shows data must render from local cache if available. Keystore/EncryptedSharedPreferences access is deferred to the background coroutine block — never in `Application.onCreate()`.
- **BFF HomeResponse is always persisted to Room** — never held in memory only. Next cold start must render data from the previous session.
- **Shimmer has a hard 2-second timeout** — `HomeViewModel` enforces this explicitly. BFF latency is never assumed to meet the guarantee on its own.
- **Diary is local-only.** No diary data (bills, ledger entries, customers) is ever sent to the backend. Room is the sole store.
- **HTTP 410 → UpdateRequiredScreen** (non-dismissible) — handled by `VersionInterceptor` + `ApiErrorMapper` before any other error path. Never retried. 

### OkHttpClient Interceptor Registration Order (NetworkModule)

Interceptors MUST be registered in this exact order:

```
1. VersionInterceptor — sets Accept-Version: v1; surfaces HTTP 410 as VersionDeprecated (FIRST; never retried)
2. AuthInterceptor — Bearer token; 401 refresh + retry (single attempt, synchronized(refreshLock))
3. AiQuotaInterceptor — reads X-Ai-Quota-* response headers → PreferenceStore (pass-through)
4. LogRedactionInterceptor — strips Authorization + Set-Cookie from logs
5. HttpLoggingInterceptor — debug builds only
```

`@Named("s3")` OkHttpClient for presigned S3 uploads **MUST NOT** include `AuthInterceptor` — presigned URLs reject the `Authorization` header.

### Entity Ownership (Room) 

| Entity | Canonical package | Redirect stub location |
|---|---|---|
| `RateEntity` | `data/local/entity/` | — |
| `HomeEntity` | `data/local/entity/` | — |
| `AlertEntity` | `data/local/entity/` | — |
| `DesignEntity` | `data/local/entity/` | — |
| `BillEntity` + `BillFts` | `feature/diary/data/local/` | `data/local/entity/BillEntity.kt` (empty stub — do not implement) |
| `CustomerEntity` + `CustomerFts` | `feature/diary/data/local/` | `data/local/entity/CustomerEntity.kt` (empty stub — do not implement) |
| `LedgerEntryEntity` | `feature/diary/data/local/` | `data/local/entity/LedgerEntryEntity.kt` (empty stub — do not implement) |

`AppDatabase` imports all entities from their canonical packages. Redirect stubs are documentation only.

---

## Android — Root

```
mahaswarna_android/
├── .github/
│ └── workflows/
│ ├── ci.yml # lint → test → build → distribute
│ └── release.yml # Play Store internal track upload (triggered on v* tags;
│ # signs AAB with keystore secrets, uploads via
│ # r0adkll/upload-google-play)
├── app/
│ ├── src/
│ │ ├── main/
│ │ │ ├── AndroidManifest.xml
│ │ │ ├── java/com/mahaswarna/
│ │ │ └── res/
│ │ │ ├── drawable/
│ │ │ │ ├── ic_launcher_foreground.xml
│ │ │ │ └── ic_notification.xml
│ │ │ ├── values/
│ │ │ │ ├── colors.xml
│ │ │ │ └── strings.xml
│ │ │ └── xml/
│ │ │ ├── network_security_config.xml # Permits cleartext to 10.0.2.2
│ │ │ │ # for debug builds only. AndroidManifest
│ │ │ │ # references this via
│ │ │ │ # android:networkSecurityConfig=
│ │ │ │ # "@xml/network_security_config"
│ │ │ └── file_paths.xml # FileProvider paths for invoice
│ │ │ # PDF sharing. Referenced in AndroidManifest
│ │ │ # under <provider android:name=
│ │ │ # "androidx.core.content.FileProvider">
│ │ ├── test/ # JUnit5 unit tests
│ │ └── androidTest/ # Espresso / Compose UI tests
│ ├── build.gradle.kts
│ └── proguard-rules.pro
├── build.gradle.kts
├── settings.gradle.kts
├── gradle/
│ ├── libs.versions.toml # DEPENDENCY VERSIONS — aligned:
│ │ # kotlin = "2.2.20"
│ │ # ksp = "2.2.20-2.0.0" (must match kotlin exactly)
│ │ # room = "2.8.3"
│ │ # retrofit = "3.0.0"
│ │ # okhttp = "5.0.0-alpha.14" (okhttp-android artifact)
│ │ # firebaseBom = "34.0.0"
│ │ # vico = "2.x" (check patrykandpatrick/vico)
│ │ #
│ │ # FIREBASE -ktx BAN (BOM 34 bundles Kotlin extensions natively):
│ │ # ✅ firebase-analytics ❌ firebase-analytics-ktx
│ │ # ✅ firebase-crashlytics ❌ firebase-crashlytics-ktx
│ │ # ✅ firebase-messaging ❌ firebase-messaging-ktx
│ │ # ✅ firebase-auth ❌ firebase-auth-ktx
│ │ # EXCEPTION : billing-ktx IS intentional.
│ │ # Play Billing Library requires -ktx for suspend/coroutine API.
│ │ # No standalone coroutine API exists in the non-ktx artifact.
│ │ # Documented exception — do not remove.
│ └── wrapper/
├── .editorconfig
├── .gitignore
├── ktlint.gradle.kts
└── detekt.yml
```

---

## Android — Core

```
app/src/main/java/com/mahaswarna/
│
├── MahaSwarnApplication.kt # Application class. Init order (invariant):
│ # 1. NotificationChannelSetup.createChannels()
│ # BEFORE Firebase — channels must exist before
│ # any FCM message can be displayed (API 26+).
│ # 2. super.onCreate() → Hilt builds app component:
│ # NetworkModule → OkHttpClient with interceptors
│ # in required order (see Cross-Cutting Invariants)
│ # DatabaseModule → Room (async open)
│ # WsModule → WsClient singleton (not connected yet)
│ # 3. Firebase auto-init (google-services plugin handles this)
│ #
│ # TokenStore is NOT accessed here.
│ # On post-reboot cold start, first Keystore TEE access
│ # takes 50–200ms on budget devices. Calling it in
│ # onCreate() consumes the 400ms budget margin before
│ # Room even opens. Token is accessed lazily by
│ # AuthInterceptor on the first background REST call.
│ #
│ # WS lifecycle is started from MainActivity, not here.
│
├── core/
│ ├── network/
│ │ ├── ApiConstants.kt # BASE_URL = gateway :4000 (versioned /v1/), WS_URL = pricing :4002
│ │ │ # const val API_VERSION = "v1"
│ │ │ # const val BASE_URL = "https://api.mahaswarna.com/v1/"
│ │ │ # timeout constants, backoff constants, shimmer timeout (2000ms)
│ │ │ # All catalog, marketplace, and invoice routes proxy
│ │ │ # through the gateway (:4000) → intelligence service (:4003).
│ │ │ # The Android client always targets the gateway — the
│ │ │ # upstream routing to intelligence is transparent.
│ │ │
│ │ ├── RetrofitClient.kt # OkHttp 5 builder. Interceptors in required order:
│ │ │ # VersionInterceptor → AuthInterceptor →
│ │ │ # AiQuotaInterceptor → LogRedactionInterceptor →
│ │ │ # HttpLoggingInterceptor (debug only)
│ │ │ #
│ │ │ # TLS + intermediate CA public key pinning (NOT leaf):
│ │ │ # Leaf certificate pinning breaks every 90 days with
│ │ │ # Let's Encrypt. Pin the intermediate CA public key instead.
│ │ │ # OkHttp CertificatePinner config:
│ │ │ # .add("api.mahaswarna.com", "sha256/<primary_pin>")
│ │ │ # .add("api.mahaswarna.com", "sha256/<backup_pin>")
│ │ │ # Primary pin: intermediate CA of current cert chain.
│ │ │ # Backup pin: next CA or pre-generated backup key —
│ │ │ # MUST be deployed in a prior app release before
│ │ │ # the primary is rotated. A broken pin with no update
│ │ │ # path is a self-inflicted outage.
│ │ │ # Pin rotation procedure:
│ │ │ # 1. Ship new release with backup_pin added alongside primary.
│ │ │ # 2. Wait for > 90% of users on new release (Play Console).
│ │ │ # 3. Rotate server cert / CA.
│ │ │ # 4. Ship release promoting backup_pin to primary.
│ │ │ # Same pinning applies to ws.mahaswarna.com (WSS).
│ │ │ #
│ │ │ # Retrofit 3 converter:
│ │ │ # .addConverterFactory(
│ │ │ # Json.asConverterFactory("application/json".toMediaType()))
│ │ │ #
│ │ │ # @Named("s3") OkHttpClient — NO AuthInterceptor.
│ │ │ # Presigned S3 URLs reject the Authorization header.
│ │ │
│ │ ├── VersionInterceptor.kt # Sets Accept-Version: v1 on every request.
│ │ │ # Intercepts HTTP 410 → throws ApiError.VersionDeprecated.
│ │ │ # 410 is NEVER retried. MainActivity observes this event
│ │ │ # and navigates to UpdateRequiredScreen (non-dismissible).
│ │ │
│ │ ├── AuthInterceptor.kt # Adds Bearer token from TokenStore.
│ │ │ # On 401 response:
│ │ │ # synchronized(refreshLock) {
│ │ │ # // double-check: another thread may have refreshed already
│ │ │ # if (tokenStore.getAccessToken() != originalToken) {
│ │ │ # return chain.proceed(request.withNewToken())
│ │ │ # }
│ │ │ # try { sessionManager.refresh() }
│ │ │ # catch (e: Exception) {
│ │ │ # sessionManager.emitLoggedOut(); throw e
│ │ │ # }
│ │ │ # }
│ │ │ # return chain.proceed(request.withNewToken()) // single retry
│ │ │ # First access to TokenStore triggers Keystore unsealing
│ │ │ # (deferred to background coroutine, not Application.onCreate).
│ │ │
│ │ ├── AiQuotaInterceptor.kt # Reads AI quota response headers from the gateway.
│ │ │ # HEADER CONTRACT (set by gateway ai_quota_interceptor.go):
│ │ │ # X-Ai-Quota-Used: <integer>
│ │ │ # X-Ai-Quota-Limit: <integer>
│ │ │ # X-Ai-Quota-Reset-At: <unix_epoch_seconds>
│ │ │ # Implementation:
│ │ │ # val response = chain.proceed(request)
│ │ │ # val used = response.header("X-Ai-Quota-Used")?.toIntOrNull()
│ │ │ # val limit = response.header("X-Ai-Quota-Limit")?.toIntOrNull()
│ │ │ # val reset = response.header("X-Ai-Quota-Reset-At")?.toLongOrNull()
│ │ │ # if (used != null && limit != null && reset != null) {
│ │ │ # preferenceStore.setAiQuota(used, limit, reset)
│ │ │ # }
│ │ │ # return response // pass-through; does not modify body
│ │ │ # Values sourced from headers ONLY — never from response body.
│ │ │ # If headers absent (non-Gemini route): no write; retain last values.
│ │ │
│ │ ├── LogRedactionInterceptor.kt # Strips Authorization + Set-Cookie from OkHttp logs
│ │ │
│ │ ├── ApiErrorMapper.kt # Maps HTTP status + error body → typed ApiError:
│ │ │ # 410 → VersionDeprecated (handled first; never retried)
│ │ │ # 401 → Unauthorized
│ │ │ # 403 + "device_not_trusted" → DeviceNotTrusted
│ │ │ # 429 → RateLimited
│ │ │ # 503 + "rate_unavailable" → RateUnavailable
│ │ │ # 5xx → ServerError
│ │ │ # Unknown rateSource values → treat as "stale" (future-proof)
│ │ │
│ │ └── NetworkMonitor.kt # ConnectivityManager flow → isOnline StateFlow
│ │
│ ├── auth/
│ │ ├── TokenStore.kt # EncryptedSharedPreferences (AES-256) wrapper.
│ │ │ # WRITE ORDER IN saveAccessToken() — REQUIRED:
│ │ │ # Step 1: write token to EncryptedSharedPreferences (commit())
│ │ │ # Step 2: THEN write token_exists_marker file
│ │ │ # Use commit() NOT apply() — apply() is async and the marker
│ │ │ # write could race ahead of the ESP write.
│ │ │ # Order matters: if killed between steps, the token exists
│ │ │ # but marker is absent → SplashScreen routes to Login (safe).
│ │ │ # Reversed order (marker first, then token): if killed, marker
│ │ │ # exists but token doesn't → routes to Home → 401 → LoggedOut.
│ │ │ # Implementation:
│ │ │ # fun saveAccessToken(token: String) {
│ │ │ # prefs.edit().putString("access_token", token).commit()
│ │ │ # File(filesDir, "token_exists_marker").createNewFile()
│ │ │ # }
│ │ │ # clearAll(): deletes token_exists_marker + all ESP keys.
│ │ │ #
│ │ │ # PROCESS-DEATH EDGE CASE: if killed between clearSessionData()
│ │ │ # and the marker delete (OOM kill mid-logout), the marker persists.
│ │ │ # On next cold start, SplashScreen routes to Home; AuthInterceptor
│ │ │ # hits the API, receives 401, cascades to SessionEvent.LoggedOut
│ │ │ # → MainActivity navigates to Login. The shimmer auto-resolves via
│ │ │ # the 2-second NoDataAvailable timeout. ACCEPTABLE — security
│ │ │ # invariant preserved (401 fires before any data is displayed).
│ │ │
│ │ ├── JwtParser.kt # Decodes JWT payload (no verification — server is source of truth).
│ │ │ # Extracts: tier, exp
│ │ │
│ │ └── SessionManager.kt # Token lifecycle: isExpired(), shouldRefresh(), refresh()
│ │ # emitLoggedOut() → emits SessionEvent.LoggedOut
│ │ # Observed by MainActivity → clearSessionData() + navigate Login
│ │
│ ├── websocket/
│ │ ├── WsClient.kt # OkHttp 5 WebSocket wrapper.
│ │ │ # connect(token), disconnect(), send(envelope)
│ │ │ # connectionState: StateFlow<WsConnectionState>
│ │ │ # Emits: Connecting → Connected → Reconnecting →
│ │ │ # Disconnected as the socket transitions.
│ │ │ # Reconnect with exponential backoff (inline):
│ │ │ # delay: 1s → 2s → 4s → … cap 60s; reset on Connected.
│ │ │ # connect() called only after JWT confirmed valid.
│ │ │ #
│ │ │ # REQUIRED FLOW PATTERN — callbackFlow + awaitClose:
│ │ │ # All flows bridged from WebSocketListener callbacks
│ │ │ # MUST use callbackFlow { … awaitClose { } }, NOT
│ │ │ # flow { } or MutableStateFlow with direct emissions.
│ │ │ # flow { } cannot call awaitClose and will leak the
│ │ │ # socket if the collector is cancelled.
│ │ │ # Canonical pattern:
│ │ │ # fun messageFlow(): Flow<WsEnvelope> = callbackFlow {
│ │ │ # val listener = object : WebSocketListener() {
│ │ │ # override fun onMessage(ws: WebSocket, text: String) {
│ │ │ # trySend(parseEnvelope(text))
│ │ │ # }
│ │ │ # override fun onClosed(ws: WebSocket, code: Int, reason: String) {
│ │ │ # close()
│ │ │ # }
│ │ │ # override fun onFailure(ws: WebSocket, t: Throwable, r: Response?) {
│ │ │ # close(t)
│ │ │ # }
│ │ │ # }
│ │ │ # val ws = okHttpClient.newWebSocket(request, listener)
│ │ │ # awaitClose { ws.close(1000, "channel closed") }
│ │ │ # }
│ │ │
│ │ ├── WsEnvelope.kt # data class: channel (rates|alerts), payload (JsonElement)
│ │ ├── WsChannelRouter.kt # Dispatches by envelope.channel to typed flows
│ │ └── WsConnectionState.kt # sealed: Connecting | Connected | Reconnecting | Disconnected | Error
│ │ # Reconnecting: in-flight backoff attempt; triggers StaleRateBanner
│ │ # after 30s without CONNECTED state.
│ │ # Error: unrecoverable failure (e.g. TLS cert mismatch, policy
│ │ # rejection); also triggers StaleRateBanner immediately (no 30s
│ │ # grace — Error is not a transient state). Client falls back to
│ │ # polling mode until the Error is resolved or app is restarted.
│ │ # StaleRateBanner trigger condition (canonical):
│ │ # wsState is Reconnecting || Disconnected for > 30s
│ │ # OR wsState is Error (immediate, no delay)
│ │
│ ├── di/
│ │ ├── NetworkModule.kt # @Provides Retrofit, OkHttpClient (primary + @Named("s3")),
│ │ │ # all API interfaces. Interceptor order enforced here.
│ │ ├── DatabaseModule.kt # @Provides AppDatabase (Room), all DAOs.
│ │ │ # NEVER .fallbackToDestructiveMigration().
│ │ │ # .addMigrations(MIGRATION_N_N1, ...) for every schema bump.
│ │ ├── RepositoryModule.kt # @Binds repository interfaces to implementations
│ │ └── WsModule.kt # @Singleton WsClient
│ │
│ ├── storage/
│ │ ├── AppDatabase.kt # Room DB. Entities registered from canonical packages:
│ │ │ # Session-scoped: RateEntity, HomeEntity, AlertEntity, DesignEntity
│ │ │ # Diary (canonical: feature/diary/data/local/):
│ │ │ # BillEntity, BillFts, CustomerEntity, CustomerFts ,
│ │ │ # LedgerEntryEntity
│ │ │ #
│ │ │ # FTS TABLES (MUST use content-backed FTS4):
│ │ │ # @Fts4(contentEntity = BillEntity::class)
│ │ │ # class BillFts — indexes customerName, itemsSummary.
│ │ │ # @Fts4(contentEntity = CustomerEntity::class)
│ │ │ # class CustomerFts — indexes name. │ │ │ # contentEntity ensures Room keeps the FTS virtual table
│ │ │ # in sync automatically on insert/update/delete.
│ │ │ # A standalone FTS4 table (no contentEntity) would fall
│ │ │ # out of sync on any BillEntity update (e.g. pdfLocalUri
│ │ │ # written by ReGenerateInvoiceUseCase).
│ │ │ #
│ │ │ # clearSessionData():
│ │ │ # Clears ONLY: RateEntity, HomeEntity, AlertEntity, DesignEntity
│ │ │ # MUST NOT touch: BillEntity, LedgerEntryEntity, CustomerEntity
│ │ │ # (Diary tables are local-only and unrecoverable).
│ │ │ # Called on logout / token expiry.
│ │ │ # clearAll() — full wipe of all tables including Diary.
│ │ │ # Called ONLY from DeleteAccountUseCase after server
│ │ │ # confirms 204 on DELETE /user/account.
│ │ │ #
│ │ │ # ROOM MIGRATION POLICY — NON-NEGOTIABLE:
│ │ │ # NEVER call .fallbackToDestructiveMigration().
│ │ │ # Diary tables are local-only and unrecoverable — a
│ │ │ # destructive migration silently wipes a jeweller's entire
│ │ │ # transaction history with no recourse.
│ │ │ # Every schema version bump MUST have an explicit Migration:
│ │ │ # val MIGRATION_1_2 = object : Migration(1, 2) {
│ │ │ # override fun migrate(db: SupportSQLiteDatabase) {
│ │ │ # db.execSQL("ALTER TABLE BillEntity ADD COLUMN ...")
│ │ │ # }
│ │ │ # }
│ │ │ # Builder: Room.databaseBuilder(...)
│ │ │ # .addMigrations(MIGRATION_1_2, ...)
│ │ │ # // NO .fallbackToDestructiveMigration()
│ │ │ # .build()
│ │ │ #
│ │ │ # DIARY MIGRATION SAFETY — REQUIRED:
│ │ │ # Every Room schema migration MUST include a @Migration test
│ │ │ # that asserts row counts for all three Diary tables are
│ │ │ # identical before and after migration:
│ │ │ # @Test fun migration_N_to_N1_preservesDiary() {
│ │ │ # val db = helper.createDatabase(TEST_DB, N)
│ │ │ # // insert fixture rows into bill, ledger, customer
│ │ │ # db.close()
│ │ │ # val migrated = helper.runMigrationsAndValidate(
│ │ │ # TEST_DB, N+1, true, MIGRATION_N_N1)
│ │ │ # assertEquals(expectedBillCount,
│ │ │ # migrated.query("SELECT COUNT(*) FROM BillEntity").use { ... })
│ │ │ # // repeat for LedgerEntryEntity, CustomerEntity
│ │ │ # }
│ │ │ #
│ │ │ # DIARY TABLE GROWTH: Diary tables grow unboundedly.
│ │ │ # Schedule a periodic SQLite VACUUM after large deletes:
│ │ │ # appDatabase.openHelper.writableDatabase.execSQL("VACUUM")
│ │ │
│ │ └── PreferenceStore.kt # DataStore<Preferences> for non-sensitive prefs.
│ │ # AI QUOTA FIELDS (written by AiQuotaInterceptor):
│ │ # aiQuotaUsed: Int (requests used this window)
│ │ # aiQuotaLimit: Int (max requests per window)
│ │ # aiQuotaResetAt: Long (unix epoch seconds)
│ │ # fun setAiQuota(used: Int, limit: Int, resetAt: Long)
│ │ # fun getAiQuotaFlow(): Flow<AiQuotaState>
│ │ # AiQuotaState: data class(used, limit, resetAt, isExhausted)
│ │ # Read by AI-gated ViewModels to show quota UI.
│ │ #
│ │ # PENDING BILL QUEUE (bill retry on Room failure):
│ │ # fun setPendingBillQueue(json: String)
│ │ # fun getPendingBillQueue(): String?
│ │ #
│ │ # CONSENT:
│ │ # fun setConsentAccepted(value: Boolean)
│ │ # fun getConsentAccepted(): Flow<Boolean>
│ │
│ └── util/
│ ├── DateTimeExt.kt # IST formatting, epoch helpers
│ ├── CurrencyExt.kt # INR formatting for gold/silver rates.
│ │ # MUST use Locale("en", "IN"), NOT Locale.US:
│ │ # Indian number system uses 2-2-3 digit grouping:
│ │ # ₹X,XX,XXX (not ₹X,XXX,XXX as in US locale).
│ │ # Correct:
│ │ # val fmt = NumberFormat.getCurrencyInstance(Locale("en", "IN"))
│ │ # fmt.format(62450.0) // → "₹62,450.00"
│ │ # Wrong (US locale): NumberFormat.getCurrencyInstance(Locale.US)
│ │ # → "$62,450.00" (wrong symbol, wrong grouping for INR)
│ │ # Wrong (manual ₹ + Locale.US grouping):
│ │ # → "₹62,450.00" looks right at this value but breaks at
│ │ #   lakhs: "$6,24,500" vs "₹6,24,500" — grouping differs.
│ │ # Always use Locale("en", "IN") for correct lakh/crore grouping.
│ │ # FORMATTING IS 100% CLIENT-SIDE:
│ │ # The backend sends raw float64 values with no currency symbol,
│ │ # grouping, or locale. Never parse a formatted string from backend.
│ └── FlowExt.kt # retryWithBackoff, throttleLatest, cachedIn helpers
```

---

## Android — Features

```
app/src/main/java/com/mahaswarna/
│
├── feature/
│ │
│ ├── auth/
│ │ ├── data/
│ │ │ ├── AuthApi.kt # POST /auth/send-otp — triggers OTP delivery
│ │ │ │ # request: { phone: String }
│ │ │ │ # response: { provider: "firebase"|"msg91" }
│ │ │ │ # POST /auth/login — OTP verify + JWT issue
│ │ │ │ # Firebase body: { phone, firebaseIdToken,
│ │ │ │ # integrityToken, cityID?, provider: "firebase" }
│ │ │ │ # MSG91 body: { phone, otp,
│ │ │ │ # integrityToken, cityID?, provider: "msg91" }
│ │ │ │ # POST /auth/refresh, POST /auth/logout
│ │ │ │ # DELETE /user/account, POST /user/consent
│ │ │ └── AuthRepository.kt # sendOtp(), login(), refresh(), logout(),
│ │ │ # logConsent(), deleteAccount()
│ │ │ # Stores tokens via TokenStore on login success.
│ │ ├── domain/
│ │ │ ├── OtpProvider.kt # sealed class: Firebase | Msg91
│ │ │ │ # Parsed from POST /auth/send-otp response.
│ │ │ │ # Drives LoginViewModel to choose verification path.
│ │ │ ├── LoginUseCase.kt
│ │ │ ├── RefreshTokenUseCase.kt
│ │ │ └── DeleteAccountUseCase.kt # multi-step: DELETE /user/account → on 204:
│ │ │ # appDatabase.clearAll() (purges Diary + session tables)
│ │ │ # tokenStore.clearAll(), FCM token invalidation
│ │ │ # navigate to Login
│ │ └── ui/
│ │ ├── SplashScreen.kt # SplashScreen API (OS-level, zero Compose frames).
│ │ │ # Routing uses token_exists_marker plain file —
│ │ │ # NOT TokenStore (which triggers Keystore TEE
│ │ │ # access: 50–200ms on budget devices post-reboot):
│ │ │ # val hasToken = File(filesDir, "token_exists_marker").exists()
│ │ │ # if (!hasToken) navigate(Route.Login); return
│ │ │ #
│ │ │ # CONSENT CHECK — DataStore is async; must NOT be read
│ │ │ # synchronously on the main thread. Use the SplashScreen
│ │ │ # ViewTreeObserver.addOnPreDrawListener pattern to hold
│ │ │ # the splash frame until the DataStore read resolves:
│ │ │ #   var isReady = false
│ │ │ #   val content = findViewById<View>(android.R.id.content)
│ │ │ #   content.viewTreeObserver.addOnPreDrawListener {
│ │ │ #     if (isReady) return@addOnPreDrawListener true
│ │ │ #     false // hold splash frame
│ │ │ #   }
│ │ │ #   lifecycleScope.launch {
│ │ │ #     val consentAccepted = prefs.getConsentAccepted().first()
│ │ │ #     if (!consentAccepted) navigate(Route.Consent)
│ │ │ #     else navigate(Route.Home)
│ │ │ #     isReady = true
│ │ │ #   }
│ │ │ # The DataStore read is fast (local file); the held frame
│ │ │ # is imperceptible. Never use runBlocking on the main thread.
│ │ │ # Never makes a network call before routing.
│ │ │
│ │ ├── LoginScreen.kt # PHONE ENTRY STATE:
│ │ │ # PhoneInputField + "Send OTP" button.
│ │ │ # On tap: LoginViewModel.sendOtp(phone)
│ │ │ # → POST /auth/send-otp
│ │ │ # → response.provider drives OTP_ENTRY state.
│ │ │ #
│ │ │ # OTP ENTRY STATE (Firebase path):
│ │ │ # Firebase SDK triggers SMS automatically.
│ │ │ # OtpInputField (6 digits) + "Verify" button.
│ │ │ # On auto-verification (instant verify on same device):
│ │ │ # ViewModel receives PhoneAuthCredential directly in
│ │ │ # onVerificationCompleted callback — skip OTP entry UI.
│ │ │ # On manual entry: user types code →
│ │ │ # PhoneAuthProvider.getCredential(verificationId, code)
│ │ │ # → getIdToken → POST /auth/login.
│ │ │ #
│ │ │ # OTP ENTRY STATE (MSG91 path):
│ │ │ # User receives SMS from MSG91 DLT-registered sender.
│ │ │ # On "Verify": LoginViewModel.verifyMsg91Otp(phone, otp).
│ │ │ #
│ │ │ # RESEND OTP:
│ │ │ # 60-second countdown timer. On tap: sendOtp(phone) again.
│ │ │ # Backend enforces max 5 resends/hour per phone;
│ │ │ # on 429: "Too many attempts — try again in 1 hour".
│ │ │ #
│ │ │ # FIREBASE ERROR HANDLING:
│ │ │ # FirebaseAuthInvalidCredentialsException → "Invalid OTP"
│ │ │ # FirebaseTooManyRequestsException → "Too many attempts"
│ │ │ # FirebaseNetworkException → "Network error — switching to SMS"
│ │ │ # → trigger MSG91 fallback: LoginViewModel.switchToMsg91()
│ │ │
│ │ ├── LoginViewModel.kt # OTP FLOW STATE MACHINE:
│ │ │ # sealed class LoginState:
│ │ │ # Idle | PhoneEntry | SendingOtp | OtpEntry(provider) |
│ │ │ # Verifying | Success | Error(message)
│ │ │ #
│ │ │ # fun sendOtp(phone: String):
│ │ │ # 1. Obtain Play Integrity token
│ │ │ # (IntegrityManager.requestIntegrityToken())
│ │ │ # store as pendingIntegrityToken
│ │ │ # 2. POST /auth/send-otp { phone }
│ │ │ # 3. on response.provider == "firebase":
│ │ │ # call startFirebaseVerification(phone)
│ │ │ # emit OtpEntry(OtpProvider.Firebase)
│ │ │ # on response.provider == "msg91":
│ │ │ # emit OtpEntry(OtpProvider.Msg91)
│ │ │ #
│ │ │ # fun startFirebaseVerification(phone: String):
│ │ │ # val options = PhoneAuthOptions.newBuilder(firebaseAuth)
│ │ │ # .setPhoneNumber("+91$phone")
│ │ │ # .setTimeout(60L, TimeUnit.SECONDS)
│ │ │ # .setActivity(activity)
│ │ │ # .setCallbacks(callbacks)
│ │ │ # .build()
│ │ │ # PhoneAuthProvider.verifyPhoneNumber(options)
│ │ │ # callbacks.onVerificationCompleted → auto-login
│ │ │ # callbacks.onCodeSent → store verificationId
│ │ │ # callbacks.onVerificationFailed → emit Error / switchToMsg91
│ │ │ #
│ │ │ # fun verifyFirebaseOtp(code: String):
│ │ │ # val credential = PhoneAuthProvider.getCredential(verificationId, code)
│ │ │ # firebaseAuth.signInWithCredential(credential)
│ │ │ # .addOnSuccessListener { result ->
│ │ │ # result.user!!.getIdToken(false)
│ │ │ # .addOnSuccessListener { tokenResult ->
│ │ │ # loginWithFirebase(phone, tokenResult.token!!)
│ │ │ # }
│ │ │ # }
│ │ │ #
│ │ │ # fun loginWithFirebase(phone: String, idToken: String):
│ │ │ # authRepo.login(phone, firebaseIdToken=idToken,
│ │ │ # integrityToken=pendingIntegrityToken, provider="firebase")
│ │ │ #
│ │ │ # fun verifyMsg91Otp(phone: String, otp: String):
│ │ │ # authRepo.login(phone, otp=otp,
│ │ │ # integrityToken=pendingIntegrityToken, provider="msg91")
│ │ │ #
│ │ │ # fun switchToMsg91(phone: String):
│ │ │ # POST /auth/send-otp { phone } — backend uses msg91 path
│ │ │ # emit OtpEntry(OtpProvider.Msg91)
│ │ │
│ │ └── ConsentScreen.kt # Full-screen route (Route.Consent) — not a dialog.
│ │ # Back navigation disabled. Shown once after first login.
│ │ # Displays Privacy Policy, Terms of Service, AI Disclaimer.
│ │ # "I Agree" → POST /user/consent (idempotent) →
│ │ # PreferenceStore.setConsentAccepted(true) → navigate Home.
│ │
│ ├── rates/
│ │ ├── data/
│ │ │ ├── RatesApi.kt # GET /rates/:cityID, GET /rates/:cityID/history
│ │ │ ├── RatesRemoteDataSource.kt
│ │ │ ├── RatesLocalDataSource.kt # Room DAO: cache latest + history (offline)
│ │ │ ├── RateEntity.kt # REDIRECT STUB ONLY — canonical at data/local/entity/
│ │ │ └── RatesRepository.kt # local-first: emit Room cache immediately,
│ │ │ # then refresh via REST, then WS updates.
│ │ │ # Source priority: WS push > REST pull > Room cache.
│ │ ├── domain/
│ │ │ └── Rate.kt # city, gold, silver, source enum, isStale, generatedAt
│ │ │ # isStale is sourced from backend field — NEVER from cachedAt.
│ │ │ # No use case wrappers — ViewModel calls RatesRepository directly.
│ │ └── ui/
│ │ ├── RatesDashboardScreen.kt # live gold/silver tiles; Gemini AI source indicator.
│ │ │ # Shows StaleRateBanner if rate.isStale == true
│ │ │ # OR if wsState != CONNECTED for > 30s.
│ │ │ # FAB "Calculator" → navigate(Route.Calculator(
│ │ │ # goldRate = currentGoldRate,
│ │ │ # silverRate = currentSilverRate))
│ │ │ # "Generate Bill" → navigate(Route.BillPrint(
│ │ │ # goldRate = currentGoldRate,
│ │ │ # silverRate = currentSilverRate))
│ │ │ # Both nav args are the live WS rate at time of tap.
│ │ ├── RatesDashboardViewModel.kt# UiState: rates, isStale, wsState.
│ │ │ # Emits StaleRateBanner trigger when
│ │ │ # wsState == Reconnecting|Disconnected for > 30s,
│ │ │ # OR wsState == Error (immediate, no 30s grace).
│ │ ├── RateHistoryScreen.kt # Vico line chart (compose-m3)
│ │ └── CityPickerBottomSheet.kt
│ │
│ ├── calculator/
│ │ ├── domain/
│ │ │ ├── CalculatorMode.kt # enum: BUY | SELL
│ │ │ │ # BUY: shopkeeper purchases raw metal from supplier/customer.
│ │ │ │ # formula: purchaseValue = weightGrams × ratePerGram
│ │ │ │ # GST: editable, default 0% (no GST liability when buying
│ │ │ │ # from unregistered individual; set 3% for registered supplier)
│ │ │ │ # SELL: shopkeeper sells jewellery to customer.
│ │ │ │ # formula: metalValue + makingCharges + GST
│ │ │ │ # GST: editable, default 3% (standard jewellery GST rate)
│ │ │ ├── CalculatorInput.kt # mode: CalculatorMode (default SELL)
│ │ │ │ # metalType: MetalType (GOLD | SILVER)
│ │ │ │ # weightGrams: Double
│ │ │ │ # ratePerGram: Double (pre-filled from nav arg, editable)
│ │ │ │ # makingChargesPercent: Double (SELL only, default 0)
│ │ │ │ # makingChargesFlat: Double (SELL only, default 0)
│ │ │ │ # makingChargesMode: PERCENT | FLAT (toggle, SELL only)
│ │ │ │ # gstPercent: Double (default 3.0 SELL / 0.0 BUY)
│ │ │ └── CalculatorResult.kt # mode: CalculatorMode (pass-through for UI rendering)
│ │ │ # metalValue: Double (weightGrams × ratePerGram)
│ │ │ # makingCharges: Double (SELL: flat + % of metalValue; BUY: 0.0)
│ │ │ # subtotal: Double (metalValue + makingCharges)
│ │ │ # gstAmount: Double (subtotal × gstPercent / 100)
│ │ │ # totalAmount: Double (subtotal + gstAmount)
│ │ │ # BUY mode result card label: "Purchase Price" instead of "Total"
│ │ └── ui/
│ │ ├── CalculatorScreen.kt # Launched from RatesDashboardScreen FAB.
│ │ │ # Nav args: goldRate: Double, silverRate: Double.
│ │ │ # Pure local — no backend call.
│ │ │ # Layout: MODE TOGGLE → METAL SELECTOR → RATE FIELD →
│ │ │ # WEIGHT FIELD → MAKING CHARGES (SELL only) → GST →
│ │ │ # RESULT CARD (live, updates as user types).
│ │ │ # GST FIELD LABEL (mode-dependent):
│ │ │ #   SELL: label = "GST (%)" — default 3%; standard jewellery rate.
│ │ │ #   BUY:  label = "GST (% if registered supplier)" — default 0%;
│ │ │ #     hint text: "Enter 3% if buying from GST-registered supplier"
│ │ │ #     This prevents shopkeepers from accidentally applying GST
│ │ │ #     on purchases from unregistered individuals (0% is correct).
│ │ │ # StaleRateBanner shown if rate.isStale == true.
│ │ └── CalculatorViewModel.kt # Pure local state — no repository, no coroutines.
│ │ # result = combine(inputFlow) { computeResult(it) }
│ │
│ ├── home/
│ │ ├── data/
│ │ │ ├── BffApi.kt # GET /bff/home
│ │ │ └── HomeRepository.kt # local-first: emit cached home data from Room,
│ │ │ # refresh via BFF on resume.
│ │ │ # REQUIRED: after every BFF fetch, persist ALL fields:
│ │ │ # homeDao.upsert(home.toRoomEntity())
│ │ │ # ratesDao.upsertAll(home.rates.map { it.toRoomEntity() })
│ │ │ # alertsDao.upsertAll(home.alerts.map { it.toRoomEntity() })
│ │ │ # prefs.setLastRefreshed(System.currentTimeMillis())
│ │ │ # Never hold HomeResponse in ViewModel memory only —
│ │ │ # the next cold start renders from Room, not from state.
│ │ ├── domain/
│ │ │ ├── HomeData.kt # aggregated: rates + alerts + shop summary
│ │ │ └── GetHomeDataUseCase.kt
│ │ └── ui/
│ │ ├── HomeScreen.kt # Renders from local cache on first frame.
│ │ │ # LoadingShimmer shown only if Room cache is empty
│ │ │ # (first install); shimmer has a 2s hard timeout.
│ │ │ # WS kill-switch polling mode (import kotlin.random.Random):
│ │ │ # lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {
│ │ │ # while (true) {
│ │ │ # delay(30_000L + Random.nextLong(-5_000L, 5_000L))
│ │ │ # homeRepo.refresh()
│ │ │ # }
│ │ │ # }
│ │ └── HomeViewModel.kt # INIT ORDER (invariant — must match exactly):
│ │ # Step 1 — shimmer timeout guard (MUST be first; assigned before
│ │ # the Room collector runs so shimmerJob?.cancel() is never null):
│ │ # shimmerJob = viewModelScope.launch {
│ │ # delay(2_000)
│ │ # if (_uiState.value is Loading) _uiState.value = NoDataAvailable
│ │ # }
│ │ # Step 2 — Room cache read (launched after shimmerJob is assigned):
│ │ # viewModelScope.launch {
│ │ # homeRepository.getCachedHome().collect { cached ->
│ │ # if (cached != null && _uiState.value is Loading) {
│ │ # _uiState.value = Success(cached)
│ │ # shimmerJob?.cancel() // safe — shimmerJob is always set before this runs
│ │ # }
│ │ # }
│ │ # }
│ │ # Steps 3–5 in single viewModelScope.launch (launched after steps 1+2):
│ │ # 3. JWT pre-warm (MUST be wrapped in try/catch;
│ │ # an uncaught exception here cancels the coroutine before
│ │ # wsClient.connect() is ever reached):
│ │ # val remainingMs = sessionManager.accessTokenRemainingMs()
│ │ # if (remainingMs < 3 * 60_000L) {
│ │ # try { authRepository.refreshToken() }
│ │ # catch (e: Exception) {
│ │ # Crashlytics.log("JWT pre-warm failed: ${e.message}")
│ │ # }
│ │ # }
│ │ # 4. wsClient.connect(tokenStore.getAccessToken())
│ │ # 5. observeHomeData().collect { data ->
│ │ # shimmerJob?.cancel()
│ │ # _uiState.value = Success(data)
│ │ # }
│ │ #
│ │ # fun retry():
│ │ # _uiState.value = Loading
│ │ # viewModelScope.launch {
│ │ # shimmerJob = launch {
│ │ # delay(2_000)
│ │ # if (_uiState.value is Loading) _uiState.value = NoDataAvailable
│ │ # }
│ │ # try {
│ │ # getHomeDataUseCase().collect { data ->
│ │ # shimmerJob?.cancel(); _uiState.value = Success(data)
│ │ # }
│ │ # } catch (e: Exception) {
│ │ # shimmerJob?.cancel(); _uiState.value = NoDataAvailable
│ │ # }
│ │ # }
│ │
│ ├── alerts/
│ │ ├── data/
│ │ │ ├── AlertsApi.kt # POST /alerts, GET /alerts, DELETE /alerts/:id
│ │ │ ├── DeviceTokenApi.kt # POST /engagement/device-token
│ │ │ └── AlertsRepository.kt # createAlert(), deleteAlert(), getAlerts()
│ │ │ # registerDeviceToken() called from MahaSwarnMessagingService
│ │ ├── domain/
│ │ │ └── Alert.kt
│ │ └── ui/
│ │ ├── AlertsScreen.kt
│ │ ├── AlertsViewModel.kt # Direct repo calls — no use case wrapper (single-step CRUD):
│ │ │ # fun createAlert(metal, threshold, direction) =
│ │ │ # viewModelScope.launch { alertsRepo.createAlert(...) }
│ │ │ # fun deleteAlert(id) =
│ │ │ # viewModelScope.launch { alertsRepo.deleteAlert(id) }
│ │ └── CreateAlertBottomSheet.kt
│ │
│ ├── billing/
│ │ ├── data/
│ │ │ ├── BillingApi.kt # POST /billing/verify, POST /billing/restore
│ │ │ ├── PlayBillingDataSource.kt # Play Billing Library 7 (billing-ktx):
│ │ │ │ # queryProductDetails, launchBillingFlow,
│ │ │ │ # consumePurchase, acknowledgePurchase
│ │ │ └── BillingRepository.kt
│ │ ├── domain/
│ │ │ ├── SubscriptionTier.kt # enum: FREE | PREMIUM | ADMIN; parsed from JWT tier claim
│ │ │ ├── VerifyReceiptUseCase.kt # Play token → POST /billing/verify → refresh JWT
│ │ │ └── RestoreSubscriptionUseCase.kt
│ │ ├── integrity/
│ │ │ └── PlayIntegrityVerifier.kt # requestIntegrityToken() → POST to backend.
│ │ │ # Called before any purchase endpoint.
│ │ │ # Also called before POST /auth/login (see LoginViewModel).
│ │ └── ui/
│ │ ├── PaywallScreen.kt # REQUIRED: apply FLAG_SECURE via DisposableEffect
│ │ │ # to prevent screenshots of paywall pricing UI:
│ │ │ # DisposableEffect(Unit) {
│ │ │ # val window = (context as Activity).window
│ │ │ # window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
│ │ │ # onDispose {
│ │ │ # window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
│ │ │ # }
│ │ │ # }
│ │ │ # clearFlags in onDispose is mandatory — failing to clear leaves
│ │ │ # FLAG_SECURE active on all subsequent screens until Activity recreated.
│ │ └── PaywallViewModel.kt
│ │
│ ├── marketplace/
│ │ ├── data/
│ │ │ ├── MarketplaceApi.kt # POST /shops, GET /shops,
│ │ │ │ # POST /shops/:id/banner
│ │ │ │ # POST /shops/:id/banner/confirm
│ │ │ │ # POST /shops/:id/invoice/generate
│ │ │ │ # body: GenerateInvoiceRequest
│ │ │ │ # response: InvoiceResponse { invoiceId, pdfBytes, generatedAt, rateSource }
│ │ │ │ #
│ │ │ │ # WIRE FORMAT — ADR-001 (DECIDED; do not reopen):
│ │ │ │ # Option A: JSON + base64-encoded PDF bytes.
│ │ │ │ # Do NOT use @Streaming + ResponseBody.
│ │ │ │ # Retrofit declaration:
│ │ │ │ # @POST("shops/{id}/invoice/generate")
│ │ │ │ # suspend fun generateInvoice(
│ │ │ │ # @Path("id") shopId: String,
│ │ │ │ # @Body request: GenerateInvoiceRequest
│ │ │ │ # ): InvoiceResponse
│ │ │ └── MarketplaceRepository.kt
│ │ ├── domain/
│ │ │ ├── Shop.kt # shopID, ownerUserID, name, address, city,
│ │ │ │ # gstNumber, phone, bannerUrl
│ │ │ ├── Invoice.kt # invoiceID, shopID, customerName, customerPhone,
│ │ │ │ # items: List<InvoiceLineItem>, subtotal, total,
│ │ │ │ # paymentMode, notes, pdfLocalUri: Uri?, generatedAt,
│ │ │ │ # rateSource: String ("live"|"stale"|"client_override"|"manual_override")
│ │ │ │ # NOTE: no pdfUrl field — PDF is not served via CDN.
│ │ │ ├── RegisterShopUseCase.kt
│ │ │ ├── UploadBannerUseCase.kt # presigned URL → S3 upload → confirm
│ │ │ └── GenerateInvoiceUseCase.kt # POST /shops/:id/invoice/generate
│ │ │ # passes goldRateOverride + silverRateOverride from nav args.
│ │ │ # Writes bytes to Context.filesDir/invoices/{invoiceId}.pdf.
│ │ │ # Exposes via FileProvider → content:// URI.
│ │ │ # rateSource warnings:
│ │ │ # "stale" → snackbar: "Invoice uses a stale rate — verify before sharing"
│ │ │ # "client_override" → snackbar: "Invoice uses the rate you saw on screen"
│ │ │ # "manual_override" → snackbar: "Invoice uses a manually set rate — verify before sharing"
│ │ │ # Unknown value → treat as "stale" warning (future-proof)
│ │ └── ui/
│ │ ├── ShopListScreen.kt
│ │ ├── RegisterShopScreen.kt
│ │ ├── ShopSettingsScreen.kt # Accessible from Profile → "My Shop".
│ │ │ # "Edit Banner" → BannerPickerScreen
│ │ │ # "Edit Details" → RegisterShopScreen (edit mode)
│ │ ├── ShopViewModel.kt
│ │ ├── BannerPickerScreen.kt # Camera (CameraX) or gallery (photo picker API).
│ │ │ # CAMERA permission: runtime request with rationale dialog.
│ │ │ # Graceful denial → gallery-only fallback.
│ │ │ # Preview shown with "Use this photo" / "Retake" actions.
│ │ │ # Confirm → UploadBannerUseCase (presigned S3 → confirm).
│ │ ├── BillPrintScreen.kt # Nav args: goldRate: Double?, silverRate: Double?
│ │ │ # RATE_UNAVAILABLE UX:
│ │ │ # Null nav arg: show editable rate input field pre-filled
│ │ │ # with 0.0; require user to enter rate before enabling
│ │ │ # "Generate Invoice" button.
│ │ │ # Backend 503 "rate_unavailable": BillPrintViewModel emits
│ │ │ # RateUnavailable → inline error card with "Enter rate manually"
│ │ │ # action that switches rate field to editable and re-submits.
│ │ │ # Customer name (required), phone (optional),
│ │ │ # line items, payment mode, notes.
│ │ │ # Running total shown live as items are filled.
│ │ │ # On success: PDF share sheet opens automatically.
│ │ │ # iTextG is NOT used anywhere — see Tech Stack.
│ │ └── BillPrintViewModel.kt # UiState: Idle | Loading | Success(localFileUri: Uri)
│ │ # | Error | RateUnavailable
│ │ #
│ │ # generateInvoice() on HTTP 503 "rate_unavailable":
│ │ # → emit RateUnavailable state
│ │ # generateInvoice() on success — ORDERING INVARIANT:
│ │ # 1. emit OpenShareSheet(localFileUri) ← always first
│ │ # 2. launch { saveBillUseCase(...) } ← fire-and-forget
│ │ # stores localFileUri + goldRateAtGeneration +
│ │ # silverRateAtGeneration in BillEntity.
│ │ # Do NOT block the share sheet on saveBillUseCase().
│ │ #
│ │ # On Room failure in saveBillUseCase():
│ │ # - Log to Crashlytics (non-fatal).
│ │ # - Write pending bill to PreferenceStore as JSON array
│ │ # under key "pending_bill_queue".
│ │ #
│ │ # QUEUE SCHEMA (versioned):
│ │ # NEVER change field names without bumping schema_version.
│ │ # A schema mismatch between writer and reader would silently
│ │ # pass 0.0 as goldRateAtGeneration on a legal document.
│ │ # {
│ │ # "schema_version": 1, ← REQUIRED
│ │ # "invoice_id": "uuid",
│ │ # "shop_id": "uuid",
│ │ # "customer_id": "uuid|null",
│ │ # "customer_name": "string",
│ │ # "items_json": "string",
│ │ # "total_amount": 0.0,
│ │ # "payment_mode": "cash|upi|card",
│ │ # "pdf_local_uri": "content://...",
│ │ # "gold_rate_at_generation": 0.0, ← CRITICAL
│ │ # "silver_rate_at_generation": 0.0, ← CRITICAL
│ │ # "generated_at": 1234567890
│ │ # }
│ │ #
│ │ # SCHEMA MIGRATION RULE:
│ │ # if gold_rate_at_generation == 0.0 or missing:
│ │ # DO NOT call saveBillUseCase silently.
│ │ # Surface: "Bill recovery failed — original rate unavailable."
│ │ # if schema_version > CURRENT_SCHEMA_VERSION (= 1):
│ │ # DO NOT parse or retry. Entry was written by a newer
│ │ # app version. Surface: "Bill recovery requires app update."
│ │ # Keep entry in queue — do NOT discard.
│ │ #
│ │ # QUEUE EVICTION POLICY (required — prevents unbounded growth):
│ │ # MAX_QUEUE_SIZE = 50 entries. If adding a new entry would
│ │ # exceed 50, drop the oldest entry and log a non-fatal to
│ │ # Crashlytics. A jeweller generating >50 invoices with a
│ │ # persistently broken Room has a larger problem than the queue.
│ │ # MAX_ENTRY_AGE = 30 days. DiaryViewModel.init() must evict
│ │ # entries with generated_at < NOW() - 30 days before retrying:
│ │ #   val cutoff = System.currentTimeMillis() / 1000 - 30 * 86_400
│ │ #   entries.filter { it.generated_at >= cutoff }
│ │ # Evicted aged entries: log non-fatal to Crashlytics with
│ │ # invoice_id so the team can investigate persistent Room failures.
│ │ # schema_version > CURRENT_SCHEMA_VERSION entries are exempt
│ │ # from age eviction — keep them until the user updates the app.
│ │ #
│ │ # DiaryViewModel.init() checks getPendingBillQueue() on every
│ │ # launch and retries saveBillUseCase() for each queued entry.
│ │ # On Room success: remove entry from queue.
│ │ # On repeated failure: surface "Some bills failed to save" banner.
│ │
│ ├── catalog/
│ │ ├── data/
│ │ │ ├── CatalogApi.kt # GET /catalog/search?q=&region=&page=&limit=
│ │ │ │ # GET /catalog/recommend?region=&page=&limit=
│ │ │ │ # POST /catalog/image-search { imageB64, region }
│ │ │ │ # ⚠️ NOT IMPLEMENTED: endpoint does not yet exist
│ │ │ │ # in the backend core service. Declaration is a placeholder.
│ │ │ │ # Do not call until backend ships the endpoint.
│ │ │ │ # Gated behind killSwitchImageSearch (NOT killSwitchCatalog).
│ │ │ │ # killSwitchImageSearch defaults to TRUE (blocked).
│ │ │ └── CatalogRepository.kt # PAGING STRATEGY: Paging 3 RemoteMediator pattern.
│ │ │ # Room holds last N results as offline browse cache.
│ │ │ # RemoteMediator fetches next page from server → appends to Room.
│ │ │ # Allows full server-side pagination, not just cached N items.
│ │ ├── domain/
│ │ │ ├── Design.kt
│ │ │ ├── SearchDesignUseCase.kt
│ │ │ └── ImageSearchUseCase.kt # bitmap → base64 → POST image-search
│ │ │ # ⚠️ NOT IMPLEMENTED. Gate with:
│ │ │ # if (flags.killSwitchImageSearch) return
│ │ │ # on timeout (>5s) or network error:
│ │ │ # → emit ImageSearchState.Error("Unable to search — try again")
│ │ │ # on empty result: → emit ImageSearchState.Empty
│ │ └── ui/
│ │ ├── CatalogScreen.kt # search bar + LazyVerticalGrid
│ │ ├── CatalogViewModel.kt # Paging 3 RemoteMediator + search debounce.
│ │ │ # Pager(config = PagingConfig(pageSize = 20),
│ │ │ # remoteMediator = CatalogRemoteMediator(...),
│ │ │ # pagingSourceFactory = { catalogDao.pagingSource() })
│ │ ├── DesignDetailScreen.kt
│ │ └── ImageSearchScreen.kt # Disabled via killSwitchImageSearch=true.
│ │ # Catalog search/recommend remain fully functional.
│ │
│ ├── flags/
│ │ ├── data/
│ │ │ ├── FlagsApi.kt # GET /config/feature-flags
│ │ │ └── FlagsRepository.kt # Refresh on app resume; cache in DataStore.
│ │ │ # getFlags() returns localCache.read() ?: DEFAULT_FLAGS
│ │ │ # DEFAULT_FLAGS = FeatureFlags(
│ │ │ # flags = mapOf("ai_enabled" to true,
│ │ │ # "shop_enabled" to true, "ws_enabled" to true,
│ │ │ # "payments_enabled" to true, "catalog_enabled" to true),
│ │ │ # killSwitch = mapOf("ai" to false, "ws" to false,
│ │ │ # "payments" to false, "catalog" to false,
│ │ │ # "image_search" to true)) // ← true = blocked
│ │ │ # No FlagGate wrapper — use standard Kotlin `if` inline
│ │ │ # in each Composable. A wrapper around `if` adds a file
│ │ │ # with no safety or readability gain.
│ │ └── domain/
│ │ └── FeatureFlags.kt # aiEnabled, shopEnabled, wsEnabled, paymentsEnabled,
│ │ # catalogEnabled, killSwitchAi, killSwitchWs,
│ │ # killSwitchPayments, killSwitchCatalog,
│ │ # killSwitchImageSearch (default true — backend not implemented)
│ │
│ ├── diary/
│ │ # LOCAL-ONLY FEATURE — no network calls, no backend API, Room only.
│ │ # Three sub-sections: Bills, Ledger, Customers.
│ │ # BillPrint auto-saves to Diary on successful invoice generation.
│ │ ├── data/
│ │ │ ├── local/
│ │ │ │ ├── DiaryDao.kt # CANONICAL location.
│ │ │ │ │ # data/local/dao/DiaryDao.kt is a redirect stub.
│ │ │ │ ├── BillEntity.kt # CANONICAL. Includes BillFts.
│ │ │ │ │ # billId (UUID), shopId, customerId (FK, nullable),
│ │ │ │ │ # customerName, invoiceId,
│ │ │ │ │ # pdfLocalUri (TEXT): content:// URI in filesDir/invoices/.
│ │ │ │ │ # Null if file deleted (app data cleared).
│ │ │ │ │ # BillsTab shows "PDF unavailable — Regenerate" chip.
│ │ │ │ │ # totalAmount, paymentMode,
│ │ │ │ │ # itemsSummary (TEXT): human-readable, indexed by BillFts.
│ │ │ │ │ # itemsJson (TEXT): full serialised List<InvoiceLineItemDto>
│ │ │ │ │ # for PDF regeneration.
│ │ │ │ │ # goldRateAtGeneration (REAL): exact gold rate per gram
│ │ │ │ │ # used when invoice was first generated. CRITICAL for
│ │ │ │ │ # ReGenerateInvoiceUseCase — ensures regenerated PDF is
│ │ │ │ │ # numerically identical to original.
│ │ │ │ │ # silverRateAtGeneration (REAL): same for silver.
│ │ │ │ │ # generatedAt (IST epoch ms), savedAt
│ │ │ │ ├── CustomerEntity.kt # CANONICAL. Includes CustomerFts .
│ │ │ │ │ # customerId (UUID), name, phone (nullable),
│ │ │ │ │ # shopId, createdAt, lastTransactionAt
│ │ │ │ └── LedgerEntryEntity.kt # CANONICAL.
│ │ │ │ # entryId (UUID), customerId (FK), shopId,
│ │ │ │ # entryType (LEND | BORROW | PAYMENT | RECEIPT),
│ │ │ │ # description, itemPurchased (TEXT, nullable),
│ │ │ │ # amount, date (IST epoch ms), notes
│ │ │ └── DiaryRepository.kt
│ │ ├── domain/
│ │ │ ├── DiaryBill.kt
│ │ │ ├── LedgerEntry.kt
│ │ │ ├── LedgerSummary.kt
│ │ │ ├── Customer.kt
│ │ │ ├── SaveBillUseCase.kt # getOrCreateCustomer → insertBill → insertLedgerEntry(LEND)
│ │ │ │ # all in a single Room transaction.
│ │ │ ├── ReGenerateInvoiceUseCase.kt # Triggered when BillEntity.pdfLocalUri is null.
│ │ │ │ # (File deleted / app data cleared — NOT CDN expiry.)
│ │ │ │ # REQUIRED GUARD before calling backend:
│ │ │ │ # if (bill.goldRateAtGeneration <= 0.0 ||
│ │ │ │ # bill.silverRateAtGeneration <= 0.0) {
│ │ │ │ # return Result.Failure(
│ │ │ │ # ReGenError.OriginalRateUnavailable(
│ │ │ │ # "Original rate unavailable — regeneration will use " +
│ │ │ │ # "today's live rate. The total may differ. Proceed?"))
│ │ │ │ # // UI shows confirmation dialog before re-calling
│ │ │ │ # // with userConfirmedLiveRate = true.
│ │ │ │ # }
│ │ │ │ # Re-calls POST /shops/:id/invoice/generate with:
│ │ │ │ # itemsJson, goldRateOverride = goldRateAtGeneration,
│ │ │ │ # silverRateOverride = silverRateAtGeneration.
│ │ │ │ # Forces RateSource = "client_override" (not live rate).
│ │ │ │ # Writes new PDF bytes to local storage.
│ │ │ │ # Updates BillEntity.pdfLocalUri in Room.
│ │ │ ├── AddLedgerEntryUseCase.kt
│ │ │ ├── GetCustomerLedgerUseCase.kt
│ │ │ ├── SearchBillsUseCase.kt # FTS4 via BillFts
│ │ │ └── SearchCustomersUseCase.kt # FTS4 via CustomerFts │ │ └── ui/
│ │ ├── DiaryScreen.kt
│ │ ├── DiaryViewModel.kt # Checks PreferenceStore.getPendingBillQueue() on init.
│ │ │ # Retries saveBillUseCase() for each queued entry.
│ │ │ # DiaryRepository.updateCustomer() called directly for
│ │ │ # single-step updates with no domain logic.
│ │ ├── BillsTab.kt # Shows "PDF unavailable — Regenerate" chip when pdfLocalUri null
│ │ ├── LedgerTab.kt
│ │ ├── CustomersTab.kt
│ │ ├── CustomerLedgerDetailScreen.kt
│ │ ├── AddLedgerEntryBottomSheet.kt
│ │ └── CustomerDetailScreen.kt
│ │
│ └── profile/ # 
│ └── ui/
│ ├── ProfileScreen.kt # Entry: My Shop → ShopSettings, Subscription status,
│ │ # Logout, Delete Account.
│ ├── ProfileViewModel.kt # Exposes: currentTier (from JWT), shopRegistered,
│ │ # logout/delete actions.
│ └── UpdateRequiredScreen.kt # Non-dismissible. Shown on HTTP 410 (VersionDeprecated).
│ # Back navigation disabled.
│ # Deep-links to Play Store listing.
│ # Triggered: ApiErrorMapper → MainActivity observes VersionDeprecated.
│
└── notification/
 ├── MahaSwarnMessagingService.kt # FirebaseMessagingService:
 │ # onNewToken → alertsRepository.registerDeviceToken(token)
 │ # (direct repository call — single upsert, no domain logic)
 │ # onMessageReceived → build + show notification
 │ #
 │ # FCM DATA PAYLOAD CONTRACT (set by deliver_alert_usecase.go):
 │ # {
 │ # "type": "price_alert",
 │ # "metal": "gold" | "silver",
 │ # "direction": "above" | "below",
 │ # "threshold": "62000", // string representation of Float
 │ # "city_id": "mumbai",
 │ # "screen": "rates" // deep-link target
 │ # }
 │ # onMessageReceived implementation:
 │ # val type = remoteMessage.data["type"]
 │ # val screen = remoteMessage.data["screen"]
 │ # val metal = remoteMessage.data["metal"]
 │ # val threshold = remoteMessage.data["threshold"]
 │ # val intent = Intent(this, MainActivity::class.java).apply {
 │ # putExtra("deep_link_screen", screen)
 │ # putExtra("deep_link_metal", metal)
 │ # flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
 │ # }
 │ # val pendingIntent = PendingIntent.getActivity(
 │ # this, 0, intent,
 │ # PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
 │ # val notification = NotificationCompat.Builder(
 │ # this, CHANNEL_ID_PRICE_ALERTS)
 │ # .setContentTitle("Gold Rate Alert")
 │ # .setContentText("Rate crossed ₹$threshold")
 │ # .setSmallIcon(R.drawable.ic_notification)
 │ # .setContentIntent(pendingIntent)
 │ # .setAutoCancel(true)
 │ # .build()
 │ # NotificationManagerCompat.from(this).notify(notifId, notification)
 │ # MainActivity.onCreate / onNewIntent reads deep_link_screen
 │ # extra and navigates to Route.Rates if screen == "rates".
 └── NotificationChannelSetup.kt # Creates "Price Alerts" channel on app start.
 # REQUIRED — Android 8+ (API 26+) silently drops all
 # notifications if the channel does not exist.
 # Called from MahaSwarnApplication.onCreate() BEFORE
 # Firebase initialisation.
 # const val CHANNEL_ID_PRICE_ALERTS = "price_alerts"
 # NotificationChannel(
 # CHANNEL_ID_PRICE_ALERTS,
 # "Price Alerts",
 # NotificationManager.IMPORTANCE_HIGH
 # ).apply {
 # description = "Gold and silver price threshold alerts"
 # enableVibration(true)
 # enableLights(true)
 # }
 # Compatible with pre-API-26 via NotificationManagerCompat.
```

---

## Android — Data Layer

```
app/src/main/java/com/mahaswarna/data/
│
├── local/
│ ├── dao/
│ │ ├── RateDao.kt # upsertRate(), getLatest(cityId), getHistory(cityId), clearAll()
│ │ ├── AlertDao.kt # clearAll() used by AppDatabase.clearSessionData()
│ │ ├── HomeDao.kt # upsert(HomeEntity), clearAll()
│ │ ├── DesignDao.kt # catalog offline cache, clearAll()
│ │ └── DiaryDao.kt # REDIRECT STUB — canonical at feature/diary/data/local/DiaryDao.kt
│ └── entity/
│ ├── RateEntity.kt # CANONICAL.
│ │ # cityId, gold, silver, source, generatedAt,
│ │ # isStale (from backend field — NEVER computed from cachedAt),
│ │ # cachedAt
│ ├── HomeEntity.kt # Serialised HomeResponse snapshot; cachedAt timestamp
│ ├── AlertEntity.kt
│ ├── DesignEntity.kt # Catalog offline cache
│ ├── BillEntity.kt # REDIRECT STUB — canonical at feature/diary/data/local/
│ ├── LedgerEntryEntity.kt # REDIRECT STUB — canonical at feature/diary/data/local/
│ └── CustomerEntity.kt # REDIRECT STUB — canonical at feature/diary/data/local/
│
├── remote/
│ └── dto/
│ ├── RateDto.kt # mirrors rates_dto.go; includes stale: Boolean
│ ├── AuthDto.kt
│ ├── BillingDto.kt
│ ├── AlertDto.kt
│ ├── ShopDto.kt
│ ├── FlagsDto.kt # includes params: Map<String, Double> for rate_sanity_threshold_pct
│ ├── CatalogDto.kt
│ ├── BffDto.kt # HomeResponse
│ │ # Fields: rates, alerts, shopSummary
│ │ # _degraded: Boolean? (optional — absent means false)
│ │ #   Set by gateway home_aggregator.go when any upstream
│ │ #   (pricing or core/alerts) times out and stale cache
│ │ #   is served instead. Client handling (HomeViewModel):
│ │ #     if (homeResponse.degraded == true) show StaleRateBanner
│ │ #   Kotlin field (use @SerialName + default false):
│ │ #     @SerialName("_degraded") val degraded: Boolean = false
│ │ #   Do NOT persist _degraded to Room — it is a transient
│ │ #   delivery signal, not a property of the rate data itself.
│ ├── InvoiceDto.kt # GenerateInvoiceRequest + InvoiceResponse (ADR-001: JSON+base64)
│ │ # GenerateInvoiceRequest fields:
│ │ # shopId, customerName, customerPhone?,
│ │ # items: List<InvoiceLineItemDto>, paymentMode, notes?,
│ │ # goldRateOverride: Double?, // 0.0 or absent → backend uses live
│ │ # silverRateOverride: Double?
│ │ # InvoiceResponse fields:
│ │ # invoiceId: String
│ │ # pdfBytes: ByteArray // base64-decoded by kotlinx.serialization
│ │ # generatedAt: String // ISO-8601 IST
│ │ # rateSource: String // "live"|"stale"|"client_override"|"manual_override"
│ │ # Kotlin:
│ │ # @Serializable
│ │ # data class InvoiceResponse(
│ │ # @SerialName("invoice_id") val invoiceId: String,
│ │ # @SerialName("pdf_bytes") val pdfBytes: ByteArray,
│ │ # @SerialName("generated_at") val generatedAt: String,
│ │ # @SerialName("rate_source") val rateSource: String
│ │ # )
│ └── ComplianceDto.kt # DeleteAccountRequest, ConsentLogRequest
│ # mirrors compliance_dto.go
│
└── mapper/
 ├── RateMapper.kt
 ├── AlertMapper.kt
 ├── HomeMapper.kt # HomeResponse → HomeEntity + RateEntity list
 ├── DesignMapper.kt
 ├── SubscriptionMapper.kt
 └── InvoiceMapper.kt
```

---

## Android — UI/Compose

```
app/src/main/java/com/mahaswarna/ui/
│
├── theme/
│ ├── Color.kt # MahaSwarna gold + charcoal palette
│ ├── Type.kt # Noto Serif (headings) + Roboto (body)
│ ├── Shape.kt
│ └── Theme.kt # MahaSwarnTheme: light + dark
│
├── components/
│ ├── GoldRateTile.kt # rate value + delta badge + source indicator
│ ├── StaleRateBanner.kt # shown when rate.isStale == true
│ │ # OR when wsState != Connected for > 30s
│ │ # Always shown in WS kill-switch polling mode.
│ ├── ShopBannerHeader.kt # shown at top of HomeScreen for shopkeeper users
│ ├── SubscriptionBadge.kt # FREE / PREMIUM chip
│ ├── LoadingShimmer.kt # skeleton placeholder (first-install only; 2s max)
│ ├── ErrorRetryCard.kt
│ └── DesignCard.kt # CDN image + title + metal badge
│
├── navigation/
│ ├── AppNavGraph.kt # NavHost with all routes.
│ │ # navController hoisted in MainActivity.setContent —
│ │ # NOT inside AppNavGraph.
│ │ # Back-stack rules:
│ │ # Calculator → back → RatesDashboard (NOT Home)
│ │ # BillPrint → back → RatesDashboard or Calculator
│ │ # SessionEvent.LoggedOut observed here → navigate Login
│ │ # ApiError.VersionDeprecated → navigate UpdateRequiredScreen │ │ #
│ │ # Routes and nav args:
│ │ # Route.Home
│ │ # Route.Consent (shown once after first login)
│ │ # Route.Rates
│ │ # Route.Calculator(goldRate: Double, silverRate: Double)
│ │ # Route.BillPrint(goldRate: Double?, silverRate: Double?)
│ │ # Route.Catalog
│ │ # Route.Diary
│ │ # Route.Profile
│ │ # Route.CustomerLedgerDetail(customerId: String)
│ │ # Route.CustomerDetail(customerId: String)
│ │ # Route.ShopSettings
│ │ # Route.BannerPicker(shopId: String)
│ │ # Route.RegisterShop / Route.EditShop(shopId)
│ │ # Route.UpdateRequired (non-dismissible, on 410)
│ ├── BottomNavBar.kt # Home | Rates | Catalog | Diary | Profile
│ └── NavRoutes.kt # sealed class Route with typed route definitions
│
└── MainActivity.kt # Single-activity. SplashScreen API (OS-level, zero Compose frames).
 # setContent {} immediately renders from local cache.
 # Observes SessionEvent.LoggedOut → clearSessionData() + navigate Login.
 # Observes ApiError.VersionDeprecated → navigate UpdateRequiredScreen.
 # Reads deep_link_screen extra from FCM notification Intent →
 # navigate(Route.Rates) when screen == "rates".
```

---

## Shared Contracts

Android DTOs mirror `src/contracts/http/` Go structs. Any backend contract change requires updating the corresponding Android DTO.

**API VERSIONING CONTRACT:**
All gateway routes are versioned under `/v1/`. `ApiConstants.kt` defines:
```kotlin
const val API_VERSION = "v1"
const val BASE_URL = "https://api.mahaswarna.com/v1/"
```
Every Retrofit request sends `Accept-Version: v1` via `VersionInterceptor`. On `HTTP 410 Gone`: `ApiErrorMapper` maps → `ApiError.VersionDeprecated` → `MainActivity` navigates to non-dismissible `UpdateRequiredScreen` that deep-links to the Play Store. This is handled before any other error path and is never retried. ` `

Breaking changes: backend ships `/v2/` with a 90-day `/v1/` compatibility window. The Android app is updated in the corresponding release to use `BASE_URL` pointing to `/v2/`. DTO versioning: `RateDtoV2.kt` etc. coexist with `RateDto.kt` until the `/v1/` window closes.

| Backend Contract | Android DTO | Notes |
|---|---|---|
| `rates_dto.go` | `RateDto.kt` | Includes `stale: Boolean` |
| `auth_dto.go` | `AuthDto.kt` | |
| `billing_dto.go` | `BillingDto.kt` | |
| `alerts_dto.go` | `AlertDto.kt` | |
| `shop_dto.go` | `ShopDto.kt` | |
| `flags_dto.go` | `FlagsDto.kt` | Includes `params: Map<String, Double>` for `rate_sanity_threshold_pct` |
| `catalog_dto.go` | `CatalogDto.kt` | |
| `bff_dto.go` | `BffDto.kt` | HomeResponse; includes `_degraded: Boolean?` (gateway partial-failure signal; default false; not persisted to Room) |
| `invoice_dto.go` | `InvoiceDto.kt` | request adds `goldRateOverride: Double?`, `silverRateOverride: Double?`; response adds `rateSource: String` |
| `compliance_dto.go` | `ComplianceDto.kt` | `DeleteAccountRequest`, `ConsentLogRequest`; used by `DeleteAccountUseCase` and `AuthRepository.logConsent()` |

**WS Envelope format:**
```json
{ "channel": "rates | alerts", "payload": { … } }
```

---

## Observability & Analytics

```
Android:
│
├── Crash Reporting (Firebase Crashlytics)
│ ├── non-fatal: API errors logged with X-Trace-ID key
│ └── fatal: stack + device info; no PII in crash payloads
│
├── Analytics (Firebase Analytics)
│ ├── screen_view (auto)
│ ├── rate_viewed { cityId, source }
│ ├── alert_created { metal, direction }
│ ├── catalog_searched { region, resultCount }
│ ├── image_search_used { region }
│ ├── calculator_used { metalType }
│ ├── bill_generated { paymentMode }
│ ├── diary_entry_added { entryType }
│ ├── subscription_flow_started
│ └── subscription_verified
│
├── Performance (Firebase Performance)
│ ├── HTTP trace per endpoint
│ ├── WS connect time custom trace
│ └── cold_start_first_frame trace # measures Room → first Compose frame; target: < 80ms
│
└── Log Redaction
 ├── Authorization header → [REDACTED]
 └── receipt tokens → [REDACTED]
```

---

## Security

| Concern | Android |
|---|---|
| Token storage | EncryptedSharedPreferences (AES-256) |
| Network | OkHttp 5 TLS + intermediate CA public key pinning (primary + backup pin); see `RetrofitClient.kt` for rotation procedure |
| Root detection | Play Integrity |
| Device attestation | Play Integrity API (login + pre-purchase) |
| Log redaction | `LogRedactionInterceptor` |
| Diary data | Local Room only — never transmitted |
| Paywall screen | `FLAG_SECURE` via `DisposableEffect`; must `clearFlags` in `onDispose` |
| API versioning | HTTP 410 → permanent blocking screen via `VersionInterceptor` + `ApiError.VersionDeprecated` |

---

## Compliance & Permissions

**Android Permissions (`AndroidManifest.xml`):**

| Permission | Justification |
|---|---|
| `INTERNET` | REST API + WebSocket |
| `POST_NOTIFICATIONS` | Price alert delivery; runtime request with graceful denial (`Build.VERSION.SDK_INT >= TIRAMISU` guard) |
| `CAMERA` | Shop banner capture + Catalog image search; explained before prompt |
| `VIBRATE` | Push notification haptic feedback |
| `ACCESS_NETWORK_STATE` | Check connectivity before WebSocket connect attempt |

**Diary on logout vs. account deletion:**

> Logout (token expiry, manual sign-out, 401 cascade) clears auth tokens and session-scoped Room tables (rates, alerts, home cache, design) but **MUST NOT clear Diary tables**. A jeweller who is force-logged-out due to an expired refresh token should not lose months of bills and ledger entries. `SessionManager` emits `SessionEvent.LoggedOut` → `MainActivity` calls `authRepository.clearSessionData()` which clears tokens + non-Diary tables only.
>
> Diary is purged exclusively by `DeleteAccountUseCase` on confirmed account deletion.
>
> `AppDatabase` exposes two clear methods:
> - `clearSessionData()` — clears RateEntity, HomeEntity, AlertEntity, DesignEntity; preserves Diary tables
> - `clearAll()` — full wipe; called only from `DeleteAccountUseCase`

**Account Deletion (client side):**
1. Profile → Settings → Delete Account.
2. Show confirmation dialog with 30-day grace period notice.
3. Call `DELETE /user/account` via `DeleteAccountUseCase`.
4. On `204`: `appDatabase.clearAll()` (all tables including Diary) + `tokenStore.clearAll()` + FCM token invalidation.
5. Navigate to Login screen.

**Consent Logging:**
- Privacy Policy, Terms of Service, AI Disclaimer shown on first launch (Route.Consent).
- Acceptance calls `POST /user/consent` via `AuthRepository.logConsent()`.
- Idempotent: safe to re-call on re-install.
- `PreferenceStore.setConsentAccepted(true)` gates subsequent SplashScreen routing.

---

## Release & CI/CD

### Android CI (`.github/workflows/ci.yml`)

```
1. ktlint + Detekt → zero warnings
2. ./gradlew test (unit tests, JUnit5)
3. ./gradlew connectedCheck (instrumented — emulator matrix)
4. ./gradlew bundleRelease
5. Upload AAB artifact
```

### Android Release (`.github/workflows/release.yml`)

Triggered on `v*` tags. Signs AAB with keystore secrets and uploads to Play Store internal track via `r0adkll/upload-google-play`.

### Environment Configuration

| Overlay | API Base URL | WS URL | Log Level |
|---|---|---|---|
| `debug` | `http://10.0.2.2:4000/v1/` | `ws://10.0.2.2:4002` | VERBOSE |
| `staging` | `https://staging-api.mahaswarna.com/v1/` | `wss://staging-ws.mahaswarna.com` | DEBUG |
| `release` | `https://api.mahaswarna.com/v1/` | `wss://ws.mahaswarna.com` | ERROR |

> **Debug URL:** Use `http://10.0.2.2:4000/v1/` for the emulator. `localhost` on the Android emulator refers to the emulator itself, not the host machine — `10.0.2.2` is the emulator's alias for the development machine.

Android: build variants via `buildTypes` + `productFlavors`.

---

## Port Map (Backend Reference)

| Backend Service | Port | Consumed By |
|---|---|---|
| gateway | 4000 | all mobile requests + BFF aggregation |
| core | 4001 | via gateway (auth, billing, alerts, flags) |
| pricing (WS) | 4002 | WebSocket connection direct from client |
| intelligence | 4003 | via gateway (catalog, marketplace, invoices) |

> **Note:** Catalog, marketplace, and invoice routes are handled by the intelligence service (port 4003) — not by core. The gateway proxies `/catalog/*`, `/shops/*`, and `/shops/:id/invoice/*` to intelligence `:4003`. The Android client always addresses the gateway `:4000`; the upstream routing is transparent to the app.
