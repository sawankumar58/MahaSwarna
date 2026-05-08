# MahaSwarna — Product Requirements Document

> **Target readers:** Android engineer, backend engineer onboarding to the project
> **Architecture source:** `mahaswarna_frontend-architecture.md`, `mahaswarna_backend-architecture.md`

---

## 1. Executive Summary

**MahaSwarna** is a real-time gold and silver rate tracking platform built exclusively for Indian jewellers. The Android-only app (Kotlin / Jetpack Compose, API 24+) delivers live precious-metal rates, a GST-aware calculator, a jewellery catalog with AI-powered image search, shop-banner management, invoice generation, and a fully offline ledger.

**Business model:** Freemium subscription distributed via Google Play, with premium tiers unlocked through Google Play In-App Purchases (Google Play Billing Library 7). Subscription state is authoritative only from the server-issued JWT `tier` claim; the client never trusts its own purchase state.

**Infrastructure:** Four Go microservices (gateway, core, pricing, intelligence) on a single Hetzner CPX41 VPS (8 vCPU / 16 GB RAM) with Docker Compose, PostgreSQL, and a 3-node Redis Sentinel cluster. Designed for 10,000 DAU at ~$60–80/month operating cost, with a documented upgrade path to Kubernetes at ~50k DAU.

---

## 2. Goals & Non-Goals

### Goals (v1 Launch)

- Deliver live gold and silver rates for 61 Indian cities via WebSocket with a sub-2-second connect time.
- Render the first meaningful UI frame from Room cache within 400ms of app open, with no network blocking.
- Provide a fully offline GST-aware calculator and local-only ledger (Diary) that survive logout.
- Enable jewellers to generate and share PDF invoices with accurate rate sourcing and rate-override support.
- Support a dual-provider OTP login (Firebase primary / MSG91 fallback) with Play Integrity attestation.
- Deliver user-configurable price alerts via FCM push and in-app WebSocket.
- Offer a jewellery catalog with full-text search and regional recommendations; image search gated behind a kill-switch pending backend implementation.
- Reach 99.5% backend uptime within the ~$80/month infra cost ceiling.

### Non-Goals

- **iOS support** — no APNS, no App Store IAP, no Swift/Objective-C code. Not in v1 or near-term roadmap.
- **Web dashboard** — no browser-based interface.
- **B2C retail users** — the product is not a consumer-facing gold price lookup tool.
- **Multi-region backend** — single-VPS deployment only; K8s upgrade path is documented but not in scope for v1.
- **Kafka or external message queue** — pg `LISTEN/NOTIFY` is the async event bus through at least ~100k DAU.
- **Direct CDN delivery of invoice PDFs** — PDFs are delivered as base64 bytes in a JSON response and stored locally only.
- **Marketplace as a consumer discovery surface** — the `marketplace/` feature package exists to enrich the jeweller's own shop profile and power invoice generation. It is not a B2B or B2C marketplace for buyers to browse shops; that use case is out of scope.

---

## 3. User Personas

### Primary — Independent Indian Jeweller

A kirana-scale shopkeeper running a single gold/silver retail or repair shop in a Tier 2–3 Indian city. Typical device: Redmi Note or Realme C-series running Android 8–11 with 3–4 GB RAM. Uses the app daily to check live rates before quoting prices, calculate buy/sell values, generate GST invoices for customers, and maintain a basic ledger. Has limited tolerance for slow cold starts or data-loss on logout. Primarily works in Hindi or a regional language; expects INR formatting with lakh/crore grouping.

**Key sensitivities:** Accuracy of rate data; invoices must reflect the rate actually shown on screen; no surprise data loss from app updates or forced logouts.

### Secondary — Sarafa Market Trader / Bullion Dealer

Operates in a wholesale sarafa market, typically in a larger city, tracking spot rates across multiple metals throughout the trading day. Monitors rate alerts for threshold crossings and may manage multiple shop profiles. More likely to use rate-history charts and to have a stable internet connection. Higher transaction volumes mean the Diary and BillPrint features carry greater compliance weight.

**Key sensitivities:** Alert delivery latency; stale rate warnings; rate-source transparency on invoices.

---

## 4. Feature Inventory

### 4.1 Rates — Real-Time Gold/Silver Rates

**Description:** Displays live gold and silver rates for the jeweller's selected city, sourced from the Gemini AI rate generator via WebSocket. Covers 61 Indian cities. The 61-city list is a compile-time constant defined in `ApiConstants.kt` — it is not fetched from a backend endpoint. If cities are added in future, a new app release is required; this is an accepted trade-off at v1 scale. Shows a stale-rate banner when data freshness cannot be guaranteed.

**User stories:**
- As a jeweller, I want to see the current gold and silver rate for my city the moment I open the app, so I can quote customers immediately.
- As a jeweller, I want the app to warn me visually when the displayed rate may be outdated, so I never quote a stale price.
- As a jeweller, I want to switch between cities so I can compare rates across markets.
- As a jeweller, I want to see a historical rate chart so I can spot intraday trends before making a purchase.
- As a jeweller, I want the last known rate to appear even when I'm offline, so the app is always useful even with poor connectivity.

**Acceptance criteria:**
- [ ] App renders rate data from Room cache within 80ms of launch (cold_start_first_frame Firebase Performance trace).
- [ ] WebSocket connects to `wss://ws.mahaswarna.com:4002` within 1–2 seconds of app open.
- [ ] WebSocket connection is only initiated after the JWT is confirmed to have >3 minutes remaining TTL; if not, a token refresh is attempted before connecting.
- [ ] `StaleRateBanner` is shown when `rate.isStale == true` (sourced from backend `stale` field in `RateDto` — never computed from client-side `cachedAt`).
- [ ] `StaleRateBanner` is shown when WebSocket is in `RECONNECTING` or `DISCONNECTED` state for >30 seconds.
- [ ] `StaleRateBanner` is shown immediately (no grace period) when WebSocket state is `ERROR`.
- [ ] `StaleRateBanner` is shown when `HomeResponse._degraded == true` (transient BFF partial-failure signal; wire field is `_degraded`, mapped to Kotlin property `degraded` via `@SerialName("_degraded")`; this field is NOT persisted to Room).
- [ ] City picker is populated from the compile-time 61-city constant in `ApiConstants.kt`; no network call is made to fetch the city list.
- [ ] Rate history is displayed as a Vico line chart on `RateHistoryScreen`. **The frontend module requires: `RateHistoryScreen.kt` (Compose screen with Vico `com.patrykandpatrick.vico:compose-m3` line chart), `RateHistoryViewModel.kt` (calls `ratesRepository.getHistory(cityId)`), and `RatesApi.kt` must declare `GET /rates/:cityID/history`. These files must be present in the `feature/rates/` module; they are not auto-generated by the Rates dashboard and must be scaffolded explicitly.**
- [ ] INR values are formatted with Indian lakh/crore grouping using `Locale("en", "IN")`.
- [ ] When WS kill-switch is active, app polls `GET /bff/home` every 30 seconds ±5 seconds (jitter is mandatory to prevent thundering herd on resume) while the screen is foregrounded only, implemented via `lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED)` — NOT a bare `while(true)` loop which continues polling when the user navigates away. `StaleRateBanner` is shown permanently in this mode.

**Offline behaviour:** Partial. Last cached rate renders from Room immediately. WebSocket reconnects automatically with exponential backoff (1s → 2s → ... 60s cap). `StaleRateBanner` always shown when disconnected >30s.

---

### 4.2 Calculator — Weight-to-Value Computation

**Description:** A GST-aware gold and silver calculator that computes buy or sell values from weight, rate, making charges, and GST inputs. Purely local — no network call of any kind.

**User stories:**
- As a jeweller, I want to enter a weight in grams and instantly see the purchase or sale value, so I can quote prices without mental arithmetic.
- As a jeweller, I want the rate field pre-filled with the live rate I see on the dashboard, so I don't have to type it manually.
- As a jeweller, I want to toggle between BUY and SELL modes, because the GST rules differ between buying raw metal and selling jewellery.
- As a jeweller, I want to add making charges (flat or percentage) in SELL mode, so the total reflects the real selling price.
- As a jeweller, I want to adjust the GST percentage, because my rate may differ from the 3% default.

**Acceptance criteria:**
- [ ] Calculator is launchable from the `RatesDashboard` FAB with `goldRate: Double`, `silverRate: Double`, and `isStale: Boolean` as nav args.
- [ ] `StaleRateBanner` is shown inside `CalculatorScreen` when the `isStale` nav arg is `true`. If the WebSocket disconnects while the user is on `CalculatorScreen`, the nav arg will not update reactively — this is the accepted v1 behaviour.
- [ ] SELL mode: `metalValue = weightGrams × ratePerGram`; `makingCharges` = flat or % of metalValue (SELL only); `subtotal = metalValue + makingCharges`; `gstAmount = subtotal × gstPercent / 100`; `totalAmount = subtotal + gstAmount`; `gstPercent` defaults to 3.0.
- [ ] BUY mode: `metalValue = weightGrams × ratePerGram`; `makingCharges = 0`; `subtotal = metalValue`; `gstAmount = subtotal × gstPercent / 100`; `totalAmount = subtotal + gstAmount`; `gstPercent` defaults to 0.0. GST in BUY mode is applied to the metal value only (no making charges); the formula must not re-use the SELL subtotal path.
- [ ] BUY mode GST field label reads "GST (% if registered supplier)" with hint text "Enter 3% if buying from GST-registered supplier". String keys: `str/calculator_gst_label_buy` and `str/calculator_gst_hint_buy` in `strings.xml` are the canonical source — do not hardcode these strings at the call site.
- [ ] Making charges support PERCENT and FLAT modes (toggle), SELL mode only.
- [ ] Result card updates live as the user types (no submit button).
- [ ] BUY mode result card label is "Purchase Price", not "Total".
- [ ] No network call is made at any point; `CalculatorViewModel` has no repository or coroutines.
- [ ] Back navigation from Calculator returns to `RatesDashboard`, not Home.

**Offline behaviour:** Fully offline. Always functional regardless of network state.

---

### 4.3 Catalog — Jewellery Design Catalog

**Description:** A paginated jewellery design catalog with full-text search and regional recommendations. Designs are loaded via Paging 3 RemoteMediator and cached in Room for offline browsing. Image search (Gemini Vision) is implemented on the client but gated behind `killSwitchImageSearch = true` pending backend endpoint delivery. The `POST /catalog/image-search` endpoint belongs to the **intelligence service** (port 4003), not the core service — `CatalogApi.kt` has been updated to reflect this attribution.

**User stories:**
- As a jeweller, I want to browse jewellery designs recommended for my region so I can show customers popular styles.
- As a jeweller, I want to search designs by keyword so I can quickly find a specific item.
- As a jeweller, I want to browse designs I've recently seen even when I'm offline.
- As a jeweller, I want to search by photo so I can identify a design a customer shows me on their phone (when available).
- As a jeweller, I want to see how many people have viewed a design so I can gauge popularity.

**Acceptance criteria:**
- [ ] Catalog is gated by `catalogEnabled` feature flag; entry point hidden if `killSwitchCatalog == true`.
- [ ] Search queries `GET /catalog/search?q=&region=&page=&limit=` with debounce.
- [ ] Regional recommendations use `GET /catalog/recommend?region=&page=&limit=`.
- [ ] Paging 3 `RemoteMediator` pattern with `pageSize = 20`; Room holds last N results as offline cache.
- [ ] `DesignDetailScreen` calls `GET /catalog/designs/:id` on entry via `GetDesignDetailUseCase`. This call is the view-count increment mechanism — the server increments the Redis counter as a side-effect. The `view_count` displayed on `DesignDetailScreen` must come from this API response, not from Room-cached data (search/recommend list endpoints do not increment view counts). On network error or offline: render from Room cache with a "View count may be outdated" indicator. `CatalogApi.kt`'s Retrofit interface must declare this endpoint explicitly. Do not call a separate increment endpoint.
- [ ] Image search (`POST /catalog/image-search`) is entirely disabled while `killSwitchImageSearch == true` (default: true); `ImageSearchScreen` is not reachable via any navigation path. **The `POST /catalog/image-search` endpoint is served by the backend intelligence service (port 4003), proxied through the gateway. `CatalogApi.kt` targets the gateway (`BASE_URL`) — no direct port-4003 URL is used. When the kill-switch is lifted both the backend endpoint and the frontend code ship in the same release.**
- [ ] **[STAGING / kill-switch lifted only]** Image search timeout (>5s) or network error emits `ImageSearchState.Error("Unable to search — try again")`. These ACs are not testable in production while `killSwitchImageSearch == true`; they must be verified in a staging environment with the backend endpoint stubbed and the kill-switch manually set to `false`.
- [ ] **[STAGING / kill-switch lifted only]** Image search empty result emits `ImageSearchState.Empty`.
- [ ] `catalog_searched` Firebase Analytics event fires with `{ region, resultCount }` on every debounced query that completes (including empty results; `resultCount` may be 0).
- [ ] `image_search_used` event fires with `{ region }` when an image search is executed.

**Offline behaviour:** Partial. Designs cached in Room via RemoteMediator are browsable offline. Search and recommendations require network. Image search requires network.

---

### 4.4 ShopBanner — Shop Profile & Banner Management

**Description:** Allows a jeweller to register a shop profile and upload a banner image that is stored in S3-compatible object storage. Banner images undergo content moderation via Gemini Vision before being made visible. Banners are resized server-side at upload-confirm time to enforce the PDF invoice size budget.

**User stories:**
- As a jeweller, I want to create a shop profile with my name, address, GST number, and phone so my invoices carry accurate details.
- As a jeweller, I want to upload a banner photo from my camera or gallery so my shop has a branded identity in the app.
- As a jeweller, I want to preview my banner before uploading so I can confirm it looks correct.
- As a jeweller, I want to update my shop details if anything changes.
- As a jeweller, I want my banner to be rejected if it contains inappropriate content, so the platform stays professional.

**Acceptance criteria:**
- [ ] `RegisterShopScreen` collects: name, address, city, GST number, phone; all fields validated before `POST /shops`.
- [ ] `BannerPickerScreen` supports camera capture (CameraX) and gallery (photo picker API).
- [ ] CAMERA permission is requested at runtime with a rationale dialog; graceful denial falls back to gallery-only.
- [ ] Banner upload uses `UploadBannerUseCase`: presigned S3 URL → direct S3 upload → `POST /shops/:id/banner/confirm`.
- [ ] The direct S3 upload request must not include an `Authorization` header; the `@Named("s3")` OkHttpClient (which excludes `AuthInterceptor`) must be used for this request. Using the primary OkHttpClient with `AuthInterceptor` will cause S3 to reject the request with `SignatureDoesNotMatch`.
- [ ] Preview with "Use this photo" / "Retake" actions is shown before upload is confirmed.
- [ ] Content moderation (server-side Gemini Vision) rejects inappropriate banners; client surfaces the rejection error.
- [ ] `confirm_banner_upload_usecase.go` (backend) resizes the uploaded banner to a maximum of 1200×400 px before writing to CDN storage, to enforce the PDF invoice size budget. This is **step one** of a two-step resize pipeline: the PDF builder (`invoice_pdf_builder.go`) performs a **second resize** to a maximum of 600×160 px with JPEG compression at quality 80 before embedding the banner in the invoice PDF. **Raw PNG/WebP from the CDN must not be embedded directly — uncompressed banners can exceed the 500 KB PDF size budget.** Integration tests in `generate_invoice_usecase_test.go` must assert: (a) the CDN-stored image dimensions are ≤1200×400 px, (b) the in-PDF image dimensions are ≤600×160 px, (c) the in-PDF banner encoding is JPEG (not PNG or WebP), and (d) the total PDF byte size is < 500 KB.
- [ ] `ShopSettingsScreen` is accessible from Profile → "My Shop"; supports "Edit Banner" and "Edit Details" flows.
- [ ] Shop feature is gated by `shopEnabled` feature flag.

**Offline behaviour:** Network-required. Banner upload and shop registration require connectivity.

---

### 4.5 BillPrint — Invoice PDF Generation

**Description:** Generates a GST-compliant PDF invoice by sending a structured request to the backend (intelligence service). The PDF is returned as base64 bytes in a JSON envelope (ADR-001), saved to local storage, and shared via the Android share sheet. Gold/silver rate overrides and rate-source warnings are supported.

**User stories:**
- As a jeweller, I want to generate a PDF invoice for a customer sale with line items, weight, making charges, and GST, so I can share a receipt immediately.
- As a jeweller, I want the invoice to reflect the exact rate I saw on screen, so there is no discrepancy between the quoted and billed price.
- As a jeweller, I want to manually override the rate on the invoice when the live rate is unavailable, so I can still generate bills during outages.
- As a jeweller, I want the invoice PDF to open in the share sheet automatically, so I can send it via WhatsApp without extra steps.
- As a jeweller, I want the generated invoice to be saved to my Diary automatically, so I have a permanent local record.

**Acceptance criteria:**
- [ ] `BillPrintScreen` accepts nav args `goldRate: Double?`, `silverRate: Double?`, and `isStale: Boolean` (pre-filled from live WS rate and stale state at time of navigation).
- [ ] `StaleRateBanner` is shown inside `BillPrintScreen` when the `isStale` nav arg is `true`. If the WebSocket disconnects while the user is on `BillPrintScreen`, the nav arg will not update reactively — this is the accepted v1 behaviour (same policy as `CalculatorScreen`).
- [ ] When `goldRate` or `silverRate` nav args are null: the corresponding rate input field(s) are shown pre-filled with 0.0; "Generate Invoice" button is disabled until **both** `goldRate > 0` **and** `silverRate > 0` are entered. Sending 0.0 to the backend is prohibited — the backend guard is `> 0`, so 0.0 is silently treated as no override.
- [ ] Invoice request body includes `goldRateOverride` and `silverRateOverride`; backend populates `rateSource`.
- [ ] `rateSource == "live"` → no snackbar shown (normal path; rate is fresh and unoverridden).
- [ ] `rateSource == "stale"` → snackbar: "Invoice uses a stale rate — verify before sharing".
- [ ] `rateSource == "client_override"` → snackbar: "Invoice uses the rate you saw on screen".
- [ ] `rateSource == "manual_override"` → snackbar: "Invoice uses a manually set rate — verify before sharing".
- [ ] Unknown `rateSource` values (any value not in the above list, excluding `"live"`) are treated as `"stale"` and surface the stale snackbar (future-proof).
- [ ] On HTTP 503 `rate_unavailable`: `BillPrintViewModel` emits `RateUnavailable`; inline error card with "Enter rate manually" action appears.
- [ ] On HTTP 429 with error code `invoice_daily_limit_exceeded` (response body: `{ "error": "invoice_daily_limit_exceeded" }`): `ApiErrorMapper` maps this to `ApiError.InvoiceLimitExceeded`; `BillPrintViewModel` surfaces: "Invoice limit reached for today — try again tomorrow." A generic `RateLimited` (429) error from the API gateway must not display this message. **The daily limit is 60 invoices per shop (keyed by `shopID`, not `userID`); multiple authenticated owners of the same shop share the same quota.** Backend enforces this via Redis key `invoice_count:{shopID}:{YYYY-MM-DD-IST}` with TTL set to seconds-until-midnight-IST; `shopID` is taken from the URL path param, never from the JWT `sub` claim. An integration test must assert that two authenticated users on the same shop share the same daily counter (see `invoice_handler.go`).
- [ ] On success: `OpenShareSheet(localFileUri)` is emitted **before** `saveBillUseCase` is launched. The share sheet must not be blocked by the Room write — `saveBillUseCase` runs fire-and-forget in a separate coroutine after the share sheet event is emitted.
- [ ] If `saveBillUseCase` fails: the entry is written to `PreferenceStore` as a JSON object under key `pending_bill_queue` (max 50 entries, FIFO eviction with Crashlytics logging when cap exceeded). The schema is versioned (`schema_version: 1`); the `gold_rate_at_generation` and `silver_rate_at_generation` fields are required and must never be zero or absent in a written entry. **Queue age-eviction policy:** entries with `generated_at` older than 30 days are evicted on `DiaryViewModel.init()` before retry. Entries with `schema_version > CURRENT_SCHEMA_VERSION` (currently `1`) are **exempt from age eviction** — they are retained in the queue until the app is updated, because parsing them could produce corrupt data. See §4.6 for the full queue recovery logic.
- [ ] If a queued entry's `schema_version` is greater than `CURRENT_SCHEMA_VERSION` (currently `1`): `DiaryViewModel` does not attempt to parse or retry it, surfaces "Bill recovery requires app update", and retains the entry in the queue without eviction.
- [ ] If a queued entry's `gold_rate_at_generation <= 0.0` or the field is absent: `DiaryViewModel` does not call `saveBillUseCase` silently; it surfaces "Bill recovery failed — original rate unavailable" and logs a non-fatal to Crashlytics. **Note:** Use `<= 0.0` (not `== 0.0`) as the guard condition — this matches `ReGenerateInvoiceUseCase`'s guard and correctly handles any negative rate that could result from data corruption.
- [ ] PDF bytes are written to `Context.filesDir/invoices/{invoiceId}.pdf` and exposed via `FileProvider` as a `content://` URI.
- [ ] `bill_generated` Firebase Analytics event fires with `{ paymentMode }`.
- [ ] Wire format follows ADR-001: JSON wrapper with `pdfBytes: ByteArray` (base64-decoded by kotlinx.serialization). `@Streaming` / `ResponseBody` must NOT be used.
- [ ] Backend `invoice_pdf_builder.go` resizes and JPEG-compresses the shop banner image (max 600×160 px, JPEG quality 80) before embedding in the PDF. Raw PNG/WebP from CDN must not be embedded directly.

**Pending bill queue schema (versioned — do not change field names without bumping `schema_version`):**
```json
{
  "schema_version": 1,
  "invoice_id": "uuid",
  "shop_id": "uuid",
  "customer_id": "uuid|null",
  "customer_name": "string",
  "items_json": "string",
  "total_amount": 0.0,
  "payment_mode": "cash|upi|card",
  "pdf_local_uri": "content://...",
  "gold_rate_at_generation": 0.0,
  "silver_rate_at_generation": 0.0,
  "generated_at": 1234567890
}
```

**Offline behaviour:** Network-required for invoice generation. Previously generated PDFs stored in `filesDir` remain accessible offline from the Diary.

---

### 4.6 Diary — Local Ledger

**Description:** A fully local ledger comprising three sections: Bills (auto-populated from BillPrint), Customers, and Ledger entries. All data lives exclusively in Room (SQLite) and is never transmitted over the network. The Diary survives logout and is purged only on confirmed account deletion.

**User stories:**
- As a jeweller, I want all my generated bills saved automatically in one place, so I can look up past transactions without filing paper receipts.
- As a jeweller, I want to record credit/debt transactions for each customer, so I can track outstanding balances.
- As a jeweller, I want my Diary data to survive when I'm logged out due to an expired session, so I never lose months of records.
- As a jeweller, I want to search my bills and customers by name or keyword, so I can find specific transactions quickly.
- As a jeweller, I want to regenerate a PDF invoice for a past bill if the file was deleted from my phone.

**Acceptance criteria:**
- [ ] Diary has three tabs: Bills, Ledger, Customers.
- [ ] `SaveBillUseCase` runs in a single Room transaction: `getOrCreateCustomer → insertBill → insertLedgerEntry(LEND)`.
- [ ] `clearSessionData()` clears `RateEntity`, `HomeEntity`, `AlertEntity`, `DesignEntity` only; Diary tables (`BillEntity`, `CustomerEntity`, `LedgerEntryEntity`) are never touched.
- [ ] `clearAll()` (full wipe) is called exclusively from `DeleteAccountUseCase` after server returns `204`.
- [ ] Full-text search on Bills uses `BillFts`; on Customers uses `CustomerFts` (Room FTS4).
- [ ] When `BillEntity.pdfLocalUri` is null: Bills tab shows "PDF unavailable — Regenerate" chip.
- [ ] `ReGenerateInvoiceUseCase` sends `goldRateOverride = goldRateAtGeneration` to force `rateSource = "client_override"`.
- [ ] If `goldRateAtGeneration <= 0.0`: `ReGenerateInvoiceUseCase` returns `Result.Failure(ReGenError.OriginalRateUnavailable)` and the UI shows a confirmation dialog with **title** "Original rate unavailable" and **body** "Regeneration will use today's live rate. The total may differ. Proceed?" with actions "Regenerate with live rate" and "Cancel". String keys: `str/regen_dialog_title` and `str/regen_dialog_body` in `strings.xml` are the canonical source. If the user confirms, `ReGenerateInvoiceUseCase` is called without a `goldRateOverride` (backend uses its live rate); if the user cancels, no request is made.
- [ ] When the user confirms regeneration without a stored rate, the resulting invoice's `rateSource` reflects the actual rate used by the backend (e.g. `"live"` or `"stale"`) — it is never hardcoded to `"client_override"` in this path.
- [ ] On `DiaryViewModel.init()`, queued entries in `pending_bill_queue` with `generated_at` older than 30 days are evicted before retry; evicted entries are logged as non-fatal to Crashlytics with their `invoice_id`. Entries with `schema_version > CURRENT_SCHEMA_VERSION` are exempt from age eviction. **Guard condition for corrupt rates: use `<= 0.0` (not `== 0.0`) — this matches `ReGenerateInvoiceUseCase`'s guard and correctly handles any negative rate that could result from data corruption. A corrupted entry with `gold_rate_at_generation = -0.01` would pass a `== 0.0` check and produce a corrupt invoice silently.**
- [ ] Room schema migrations must never use `fallbackToDestructiveMigration()`; every version bump requires an explicit `Migration` object. Every migration test must assert row counts for all three Diary tables are identical before and after migration.
- [ ] No Diary data appears in any network request, log, or crash payload.
- [ ] `diary_entry_added` Firebase Analytics event fires with `{ entryType }`. Valid `entryType` values: `LEND`, `BORROW`, `PAYMENT`, `RECEIPT`.
- [ ] Customer name and phone are editable from `CustomerDetailScreen`; `DiaryRepository.updateCustomer()` persists changes in Room. No network call is made — this is a local-only operation. There is no backend `/customers` endpoint; `customerId` is a local Room concept only and is never sent to any API.

**Offline behaviour:** Fully offline. Always functional. Room is the sole data store; no network path exists.

---

## 5. Authentication & Onboarding

### OTP Flow

Login uses a dual-provider OTP system:

1. Client calls `POST /auth/send-otp { phone }`.
2. Backend response includes `{ provider: "firebase" | "msg91" }` based on the `otp_provider` feature flag (`firebase | msg91 | both`).
3. **Firebase path (primary):** Client calls `FirebaseAuth.verifyPhoneNumber()`. On auto-verification, `PhoneAuthCredential` is received immediately. On manual entry, user types the 6-digit code. `firebaseIdToken` is obtained from the credential and sent to `POST /auth/login` with `provider: "firebase"`.
4. **MSG91 path (fallback):** Backend delivers OTP via MSG91 (TRAI DLT-compliant Indian SMS gateway). User types the 6-digit code; client sends it to `POST /auth/login` with `provider: "msg91"`.
5. **`POST /auth/register` vs `POST /auth/login` — new-user flow:** Both endpoints handle OTP-verified phone numbers. `POST /auth/register` exists for clients that explicitly create a new account with a `cityID` parameter (stored as `users.city_id`). `POST /auth/login` runs an `INSERT ON CONFLICT DO NOTHING` upsert — it creates the user row on first call or authenticates the existing user on subsequent calls. **For v1, the Android client always calls `POST /auth/login`** (not `/auth/register`); the upsert path handles both new and returning users. `cityID` must be included in the `/auth/login` body on first login to ensure `users.city_id` is populated. Processing rules by endpoint:
   - `POST /auth/register`: `cityID` is processed and stored as `users.city_id`. For new users, this determines their default city for rate display.
   - `POST /auth/login`: `cityID` is accepted in the request body for client compatibility but is **not** re-processed for existing users (their `city_id` is already set). For first-time users whose `POST /auth/login` triggers the upsert path, the `cityID` from the login body is used. Omitting `cityID` in the login body is safe for returning users.
6. In `both` mode: Firebase is attempted first on the client. On `FirebaseNetworkException` or SDK failure, the client calls `LoginViewModel.switchToMsg91()` which re-triggers `POST /auth/send-otp`, and the backend switches to MSG91.
7. **Server-side silent fallback (distinct from client-side, infrastructure errors only):** In `both` mode, if the client sends a `firebaseIdToken` to `POST /auth/login` and the Firebase Admin SDK on the server experiences an infrastructure error (timeout, SDK internal error — not a credential verification failure), the server re-attempts verification using `Msg91OtpProvider` only if an OTP code is also present in the same request body. If no OTP is present (pure Firebase path), the server returns `401` without MSG91 retry. A successful silent fallback returns a normal JWT with no indication of which provider verified the OTP. This fallback does NOT apply when Firebase returns a definitive credential-invalid response — that always returns `401` immediately. **Note:** The Android v1 client never sends both `firebaseIdToken` and `otp` in the same request; this server-side path is a defensive infrastructure-error recovery mechanism only, not a client-triggered flow.
8. Resend OTP: 60-second countdown timer; backend enforces max 5 resends/hour per phone (returns `HTTP 429` on excess).
9. Play Integrity token is obtained via `IntegrityManager.requestIntegrityToken()` before `POST /auth/login` and included in the request body. On `HTTP 403 { "error": "device_not_trusted" }`: non-dismissible "This device is not supported" screen; no navigation to Home. On `IntegrityManager` failure: login error is surfaced; silent bypass is not permitted. **Token expiry:** if the Play Integrity token has expired by the time `/auth/login` is called (e.g. user took >10 minutes to enter the OTP), Google returns an expiry error. Backend must return `HTTP 403 { "error": "integrity_token_expired" }`; client surfaces a retry error and resets the login flow (re-initiates `POST /auth/send-otp`).

### Consent Screen

- Shown as a full-screen route (`Route.Consent`) on first launch after login. Back navigation is disabled.
- Displays: Privacy Policy, Terms of Service, AI Disclaimer.
- **Consent logging:** Acceptance calls `POST /user/consent` exactly **twice** — once with `consentType: "privacy_policy"` and once with `consentType: "tos"`. The AI Disclaimer is displayed for transparency but is **not** a separately logged consent type; no `POST /user/consent` call is made for it. Valid `consentType` values are `"privacy_policy"` and `"tos"` only — `ConsentLogRequest` must never be constructed with any other value.
- `PreferenceStore.setConsentAccepted(true)` is written on acceptance. Subsequent launches check this flag to skip the consent screen.
- Safe to re-call on re-install (idempotent by design).

### Cold-Start Routing Logic

Splash screen routing uses a plain `token_exists_marker` file in `filesDir` — **not** `TokenStore` (EncryptedSharedPreferences). On post-reboot cold start, the first Keystore TEE/StrongBox access takes 50–200ms on budget devices. Calling it at splash would consume the entire 400ms first-frame budget.

Routing sequence:
1. Check `File(filesDir, "token_exists_marker").exists()`.
2. If absent → `navigate(Route.Login)`.
3. If present → hold splash frame via `ViewTreeObserver.addOnPreDrawListener` while `DataStore` consent read resolves asynchronously.
4. If `consentAccepted == false` → `navigate(Route.Consent)`.
5. Otherwise → `navigate(Route.Home)`.

No network call is made before routing.

### OTP Provider Path Acceptance Criteria

- [ ] When `POST /auth/send-otp` responds with `{ provider: "firebase" }`, `LoginViewModel` state transitions to `OtpEntry(OtpProvider.Firebase)` and the Firebase verification flow is initiated.
- [ ] When `POST /auth/send-otp` responds with `{ provider: "msg91" }`, `LoginViewModel` state transitions to `OtpEntry(OtpProvider.Msg91)` and the manual SMS entry UI is shown.
- [ ] On `FirebaseNetworkException`, `LoginViewModel.switchToMsg91()` re-calls `POST /auth/send-otp` and state transitions to `OtpEntry(OtpProvider.Msg91)`.
- [ ] `FirebaseTooManyRequestsException` surfaces "Too many attempts" error in the UI and does **not** trigger MSG91 fallback. This is intentional product policy: the Firebase rate limit reflects legitimate SMS abuse prevention. Silently switching providers would allow the rate limit to be circumvented. Users who hit the limit must wait; no "Send via SMS" alternative is offered.
- [ ] **[Unit test required]** `LoginViewModelTest` must include a test asserting that when `FirebaseTooManyRequestsException` is thrown in `onVerificationFailed`, the ViewModel emits `Error("Too many attempts…")` and does **not** call `POST /auth/send-otp` a second time (i.e. `switchToMsg91()` is never invoked). Without this test a future refactor could silently re-introduce the MSG91 fallback path.
- [ ] On `ConsentScreen`, "I Agree" triggers exactly **two** sequential `POST /user/consent` calls: first `consentType: "privacy_policy"`, then `consentType: "tos"`. No call is made for the AI Disclaimer. `ConsentLogRequest` must never be constructed with `consentType: "ai_disclaimer"` or any value outside `{"privacy_policy", "tos"}`. A single call covering both types is incorrect — the backend requires one record per type. The count is visible in `consent_log` and is a compliance invariant.
- [ ] On `HTTP 403 { "error": "integrity_token_expired" }` from `POST /auth/login`: `LoginViewModel` emits an error message ("Session expired — please try again") and resets state to `PhoneEntry`, forcing the user to re-initiate `sendOtp()`. A fresh Play Integrity token is obtained during the next `sendOtp()` call.

---

## 6. Subscription & Billing

- IAP is implemented with **Google Play Billing Library 7** (`billing-ktx`). This is an explicit exception to the Firebase `-ktx` ban — the `-ktx` suffix is required for the coroutine/suspend API.
- Subscription tier is read exclusively from the JWT `tier` claim (`FREE | PREMIUM | ADMIN`). The client never trusts its own purchase state; `SubscriptionTier` is refreshed from the JWT after a successful `POST /billing/verify` call.
- **Purchase flow:**
  1. `PlayIntegrityVerifier.requestIntegrityToken()` is called before any purchase endpoint.
  2. `PlayBillingDataSource` queries product details and launches the billing flow.
  3. On purchase: `POST /billing/verify` sends the Play receipt token to the backend for server-side verification via the Google Play Developer API.
  4. On success: JWT is refreshed; `SubscriptionTier` is updated from the new JWT claim.
- **Restore:** `POST /billing/restore` for users switching devices or reinstalling.
- **Paywall screen (`PaywallScreen.kt`):** `FLAG_SECURE` is applied via `DisposableEffect` to prevent screenshots of paywall pricing UI. `clearFlags` must be called in `onDispose` — failure to clear leaves `FLAG_SECURE` active on all subsequent screens until the `Activity` is recreated.
- Payments feature is gated by `paymentsEnabled` feature flag; `killSwitchPayments` blocks all purchase flows immediately.

**Acceptance criteria:**
- [ ] `FLAG_SECURE` is set on `PaywallScreen` entry via `DisposableEffect(Unit)` calling `window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)`.
- [ ] `FLAG_SECURE` is cleared in `onDispose` via `window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)` when navigating away from `PaywallScreen`. Failure to clear is a regression that blocks screenshots on all subsequent screens.
- [ ] After `POST /billing/verify` succeeds, `authRepository.refreshToken()` is called, the new JWT is parsed for the `tier` claim, and `SubscriptionBadge` reflects `PREMIUM` without requiring an app restart or manual navigation.
- [ ] `subscription_flow_started` Firebase Analytics event fires when the user taps the subscription CTA.
- [ ] `subscription_verified` Firebase Analytics event fires when `POST /billing/verify` returns success.

**Restore subscription (`POST /billing/restore`):**
- [ ] Restore is triggered from the Paywall screen ("Restore purchases" action) for users reinstalling or switching devices.
- [ ] On success: JWT is refreshed; `SubscriptionTier` is updated from the new JWT `tier` claim; `SubscriptionBadge` reflects the restored tier without requiring an app restart.
- [ ] On `HTTP 404` (no active subscription found for this Google account): surface "No active subscription found for this account."
- [ ] On any other error: surface a generic retry error; do not navigate away from Paywall.
- [ ] `killSwitchPayments` blocks the restore flow in the same way it blocks the purchase flow — the "Restore purchases" action is hidden when the kill-switch is active. **Note: this kill-switch enforcement is client-side only. The backend `POST /billing/restore` endpoint does not check `kill_switch_payments` — it will accept requests even if the flag is active. The client gate is the sole enforcement layer; this is an accepted trade-off at v1 scale. If server-side gating is required in future, add a `kill_switch_payments` check in `restore_subscription_usecase.go`.**

---

## 7. Real-Time Architecture (User-Facing Requirements)

### WebSocket Connection

- Client connects to `wss://ws.mahaswarna.com:4002` (pricing service, directly — bypasses gateway per ADR-002).
- Connection is established within 1–2 seconds of app open. Per the cold-start timing budget: JWT pre-warm runs inside the T+80ms **background coroutine** block (off the main thread); the actual `WebSocket.connect()` call is made at **T+800ms** once the token is confirmed valid. "Pre-warm" means the `refreshToken()` call is awaited sequentially within that coroutine before `wsClient.connect()` is reached — it is sequential within a coroutine, not a main-thread blocking call. Do not use `runBlocking` on the main thread for this step.
- JWT is pre-warmed at T+80ms: if `accessTokenRemainingMs < 3 minutes`, `authRepository.refreshToken()` is awaited before WS connect, ensuring the token is valid for at least ~12 minutes on connection. The pre-warm call must be wrapped in `try/catch`; an uncaught exception cancels the coroutine before `wsClient.connect()` is reached.
- WebSocket reconnects with exponential backoff: 1s → 2s → 4s … capped at 60s.

**Acceptance criteria:**
- [ ] WebSocket connection is only initiated after JWT TTL is confirmed to be >3 minutes; if TTL < 3 minutes, `authRepository.refreshToken()` is called first.
- [ ] JWT pre-warm failure (network error, server error) is caught and logged to Crashlytics; WS connect proceeds regardless (the next 401 on the WS handshake will trigger a reconnect with a fresh token).

### First Frame Performance

- Target: first Compose frame rendered from Room cache within **50–80ms** of `MainActivity.onCreate()`.
- Room cache query returns in ~5–15ms; `RatesViewModel.init()` kicks off the Room query at T+10ms.
- `TokenStore` (Keystore) is never accessed before the first frame is rendered.

### Stale Banner Conditions

`StaleRateBanner` is displayed when **any** of the following are true:

- `rate.isStale == true` (sourced from the backend `stale` field in `RateDto` — never computed from client-side `cachedAt`).
- WebSocket has been in `RECONNECTING` or `DISCONNECTED` state for more than 30 seconds.
- WebSocket state is `ERROR` (immediate, no grace period). **`ERROR` is a terminal, non-transient state** — it is not retried automatically. It is distinct from `RECONNECTING` (transient backoff) and is triggered by unrecoverable failures such as a TLS certificate mismatch or a policy rejection from the server. `WsConnectionState.kt` must document this distinction; `RatesDashboardViewModel` must show the banner immediately on `ERROR` without the 30-second timer. The `ERROR` state clears only on app restart or an explicit user retry that re-calls `wsClient.connect()`.
- WS kill-switch is active (polling mode — banner shown permanently).
- BFF `HomeResponse._degraded == true` (gateway partial-failure signal; `HomeViewModel` shows `StaleRateBanner` on this condition; the `_degraded` field is wire name — Kotlin property is `degraded` via `@SerialName("_degraded")` — **not** persisted to Room — it is a transient delivery signal only).

### Alert Delivery

Price alerts are delivered via two concurrent channels:
- **FCM push:** notification with deep-link `screen: "rates"` that navigates to `RateScreen` on tap.
- **In-app WebSocket:** envelope `{ "channel": "alerts", "payload": { … } }` delivered on the live WS connection.

---

## 8. Price Alerts

Users can set threshold-based alerts per metal and direction.

### Alert Configuration

| Field | Values |
|---|---|
| Metal | `gold` \| `silver` |
| Direction | `above` \| `below` |
| Threshold | User-entered rate value (INR) |

### Alert CRUD

Alerts are managed directly from `AlertsViewModel` via `AlertsRepository` — no use case wrapper (single-step CRUD with no domain logic):

```
createAlert(metal, threshold, direction) → POST /alerts
deleteAlert(id) → DELETE /alerts/:id
getAlerts() → GET /alerts (also hydrated from BFF home response)
```

> **No edit/update endpoint exists** (`PUT /alerts/:id` is not in the architecture). Modifying an alert requires deleting it and creating a new one. This is a deliberate omission. If in-place editing is required, it must be added as a new backend endpoint and a new OQ raised.

### Acceptance Criteria

- [ ] `AlertsScreen` lists all active alerts with metal type, direction, and threshold.
- [ ] `CreateAlertBottomSheet` collects metal, direction, and threshold; calls `alertsRepo.createAlert()`.
- [ ] There is no edit or in-place update action on existing alerts. The only modification flow is delete followed by create. `AlertsScreen` must not render an edit button or swipe action.
- [ ] Delete swipe / button calls `alertsRepo.deleteAlert(id)`.
- [ ] FCM `price_alert` payload includes `{ type, metal, direction, threshold, city_id, screen }`.
- [ ] Tapping FCM notification deep-links to `Route.Rates` when `screen == "rates"`.
- [ ] In-app alert arrives on WS channel `"alerts"` while app is foregrounded.
- [ ] `alert_created` Firebase Analytics event fires with `{ metal, direction }`.

---

## 9. Notifications

### Permission

- `POST_NOTIFICATIONS` permission is requested at runtime (guarded by `Build.VERSION.SDK_INT >= TIRAMISU`).
- Graceful denial: notifications are silently skipped; the app remains fully functional.
- `NotificationChannelSetup.createChannels()` must be called from `MahaSwarnApplication.onCreate()` **before** Firebase initialisation. Channel `CHANNEL_ID_PRICE_ALERTS` uses `IMPORTANCE_HIGH` with vibration and lights enabled. **This ordering invariant must also be present as a comment inside `MahaSwarnApplication.kt` (see Android Core file structure) — a developer reading only that file must see the before-Firebase requirement without cross-referencing the PRD.**

### Deep-Link Handling

- FCM data payload: `{ "type": "price_alert", "metal": …, "direction": "above" | "below", "threshold": …, "city_id": …, "screen": "rates" }`. <!-- GAP-07 fix: added `direction` field, aligned with §8 AC and MahaSwarnMessagingService.kt contract -->
- `MahaSwarnMessagingService.onMessageReceived()` builds a notification with a `PendingIntent` carrying `deep_link_screen` as an Intent extra.
- `MainActivity.onCreate()` and `onNewIntent()` read `deep_link_screen`; when `screen == "rates"` → `navigate(Route.Rates)`.

### FCM Token Lifecycle

- `MahaSwarnMessagingService.onNewToken()` handles two cases:
  - If the user is authenticated (valid token in `TokenStore`): call `alertsRepository.registerDeviceToken(token)` → `POST /engagement/device-token` immediately.
  - If the user is **not** authenticated (no token, e.g. first install before login): store the FCM token locally in `PreferenceStore` under key `pending_fcm_token`. On the next successful `POST /auth/login` response, read and register the pending token, then clear `pending_fcm_token`.
- On account deletion: FCM token is invalidated as part of `DeleteAccountUseCase`.

**Acceptance criteria:**
- [ ] If `onNewToken` fires before the user is authenticated, the token is written to `PreferenceStore.pending_fcm_token` and registration is deferred.
- [ ] On the next successful login, `AuthRepository.login()` reads `pending_fcm_token` from `PreferenceStore`, calls `alertsRepository.registerDeviceToken()`, and clears `pending_fcm_token`.
- [ ] If `onNewToken` fires when the user is already authenticated, registration proceeds immediately without deferral.

---

## 10. Offline & Degraded-Mode Requirements

| Feature | Offline / Degraded Behaviour |
|---|---|
| Rates | Last cached rate renders from Room; `StaleRateBanner` shown when disconnected >30s |
| Calculator | Fully offline — no network dependency |
| Catalog | Cached designs (Paging 3 Room cache) browsable; search/recommend requires network |
| ShopBanner | Network-required; upload fails gracefully with error state |
| BillPrint | Network-required for generation; previously saved PDFs accessible from Diary |
| Diary | Fully offline — Room only; always functional |
| Feature Flags | Fall back to `DEFAULT_FLAGS` on first install or network failure (see below) |
| BFF Home | `_degraded: Boolean?` (wire field name) in response signals partial upstream failure; `HomeViewModel` shows `StaleRateBanner` when `_degraded == true` (Kotlin property: `degraded`); field is transient and not persisted to Room. **The BFF `GET /bff/home` response aggregates rates (city-scoped, shared) and alerts (user-scoped). Shop summary data is NOT included in the BFF response — it is fetched via a separate `GET /shops` call when the ShopBanner feature is accessed. `buildHomeResponse()` in `home_aggregator.go` merges only rates + alerts; a developer must not expect shop data to be pre-populated from the BFF cache on cold start.** |

### Feature Flag Defaults

On first install or when `GET /config/feature-flags` fails:

```
ai_enabled: true, shop_enabled: true, ws_enabled: true,
payments_enabled: true, catalog_enabled: true
killSwitch.ai: false, killSwitch.ws: false, killSwitch.payments: false,
killSwitch.catalog: false, killSwitch.image_search: true  ← blocked by default
```

### WS Kill-Switch Polling Mode

When `killSwitchWs == true`, the WS client does not connect. `HomeViewModel` polls `GET /bff/home` every 30 ±5 seconds (±5s jitter is mandatory) using `lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED)`. `StaleRateBanner` is shown permanently in this mode.

> **Kill-switch activation pre-condition (required ops step — do not skip):** At peak DAU (~1,200 concurrent users), activating `killSwitchWs` causes all clients to switch to polling mode, generating ~40 RPS against `GET /bff/home` — matching the normal BFF peak ceiling. Before toggling the flag, the backend ops team **must** raise the FREE-tier BFF rate limit to accommodate the combined polling load. This is a mandatory runbook step, not an advisory. Failure to raise the limit before activation will cause widespread HTTP 429 errors and a degraded experience for all users simultaneously. See OQ-8 for the open question on automating this as an atomic pre-condition check.

---

## 11. API Versioning & Forced Upgrade

- All gateway routes are versioned under `/v1/`. `BASE_URL = "https://api.mahaswarna.com/v1/"`.
- Every Retrofit request sends `Accept-Version: v1` via `VersionInterceptor`.
- **HTTP 410 Gone** → `ApiErrorMapper` maps to `ApiError.VersionDeprecated` → `MainActivity` navigates to `UpdateRequiredScreen` (non-dismissible; back navigation disabled). The screen deep-links directly to the Play Store. This path is never retried.
- **HTTP 400 `unsupported_api_version`** → returned by the gateway when the `Accept-Version` header value is unrecognised (e.g. a future client bug sending `v3` before the server ships it). `ApiErrorMapper` maps `400 + body.error == "unsupported_api_version"` to `ApiError.VersionDeprecated` and follows the same blocking-screen path as 410. This case is distinct from a generic 400 validation error; the body discriminator is required.
- **Breaking change procedure:** Backend ships `/v2/` routes. A 90-day `/v1/` compatibility window applies. After 90 days, `/v1/` returns `HTTP 410`. Android app updated in the corresponding release to point `BASE_URL` to `/v2/`. DTOs versioned as `RateDtoV2.kt` etc., coexisting until the `/v1/` window closes.
- Non-breaking changes (additive fields, new optional params) do not require a version bump.

**Acceptance criteria:**
- [ ] When `ApiError.VersionDeprecated` is received (via HTTP 410 **or** HTTP 400 `unsupported_api_version`), the back stack is cleared before navigating to `UpdateRequiredScreen` using `popUpTo(Route.Home) { inclusive = true }` (or equivalent root route), so the user cannot return to any previous screen via back navigation.
- [ ] `ApiErrorMapper` handles both triggers: `HTTP 410` (any body) → `ApiError.VersionDeprecated`; `HTTP 400` with `body.error == "unsupported_api_version"` → `ApiError.VersionDeprecated`. A generic 400 validation error must **not** trigger this path. **The 400 + body discriminator check must be implemented in `ApiErrorMapper.kt` (not only in `VersionInterceptor.kt`) because the 410 path is handled in the interceptor but the 400 + body path requires response body parsing which belongs in the mapper. Both paths must be covered by `ApiErrorMapperTest`.**
- [ ] Both the hardware back button and the system back gesture are suppressed on `UpdateRequiredScreen` via `BackHandler(enabled = true) { }` — the `BackHandler` alone is not sufficient without also clearing the back stack (the system back gesture can still dismiss on some Android versions if the stack is not empty).
- [ ] `UpdateRequiredScreen` deep-links to the Play Store listing via an `Intent` with `ACTION_VIEW`.

---

## 12. Security Requirements

| Concern | Requirement |
|---|---|
| Token storage | JWT access token (15-min TTL) and refresh token (30-day TTL) stored in `EncryptedSharedPreferences` (AES-256). Never in plain `SharedPreferences`. |
| Token write order | In `TokenStore.saveAccessToken()`: `EncryptedSharedPreferences.edit().putString("access_token", token).commit()` must complete **before** `token_exists_marker` is written. `commit()` (synchronous) must be used — NOT `apply()` (asynchronous). Using `apply()` creates a race where the marker is written before the token is flushed; if the process is killed between writes, the next cold start routes to Home but `AuthInterceptor` reads no token, hits `401`, and force-logs out the user. Reversed write order (marker first, then token) has the same consequence. **This invariant must be documented as a comment inside `TokenStore.kt` itself (not only here), so a developer editing that file sees the constraint without cross-referencing the PRD.** **Process-death edge case during logout:** if the process is OOM-killed between `clearSessionData()` completing and the marker file being deleted, the marker will persist on disk. On the next cold start, `SplashScreen` routes to Home; `AuthInterceptor` hits the API, receives `401`, and cascades to `SessionEvent.LoggedOut` → navigate to Login. The shimmer auto-resolves via the 2-second `NoDataAvailable` timeout. This is an accepted safe failure mode — no data is exposed before the 401 fires. QA must include a test case for this scenario. |
| Network transport | OkHttp 5 TLS + intermediate CA public key pinning (primary + backup pin). Rotation procedure documented in `RetrofitClient.kt`. |
| Root detection | Play Integrity API on login and pre-purchase. On `HTTP 403 device_not_trusted`: non-dismissible blocking screen. On `IntegrityManager` failure: login error surfaced; silent bypass not allowed. |
| Log redaction | `LogRedactionInterceptor` redacts the `Authorization` header and receipt tokens to `[REDACTED]` in all logs. No JWT, receipt token, or API key is written to any log at any level. |
| Diary data | Local Room only. Never transmitted over any network path. Never appears in crash payloads. |
| Paywall screen | `FLAG_SECURE` via `DisposableEffect`; `clearFlags` in `onDispose` is mandatory. |
| API versioning | `HTTP 410` → permanent non-dismissible `UpdateRequiredScreen` via `VersionInterceptor` + `ApiErrorMapper`. |
| Interceptor order | `VersionInterceptor` → `AuthInterceptor` → `AiQuotaInterceptor` → `LogRedactionInterceptor` → `HttpLoggingInterceptor` (debug only). S3 client must NOT include `AuthInterceptor`. |
| S3 upload client | `@Named("s3")` OkHttpClient must be used for all presigned S3 upload requests. `AuthInterceptor` must not be present on this client — presigned URLs reject the `Authorization` header with `SignatureDoesNotMatch`. |

---

## 13. Compliance & Data Privacy

### Permissions

| Permission | Justification |
|---|---|
| `INTERNET` | REST API calls + WebSocket connection |
| `POST_NOTIFICATIONS` | Price alert delivery via FCM; runtime request (API 33+); graceful denial |
| `CAMERA` | Shop banner capture (CameraX) + Catalog image search; explained to user before prompt; graceful denial falls back to gallery-only |
| `VIBRATE` | Push notification haptic feedback on price alerts |
| `ACCESS_NETWORK_STATE` | Check connectivity before WebSocket connect attempt |

### Consent Logging

- `POST /user/consent` is called on first acceptance with exactly two separate calls: `consentType: "privacy_policy"` and `consentType: "tos"`.
- **Valid `consentType` values: `"privacy_policy"` and `"tos"` only.** The AI Disclaimer is displayed on the consent screen for transparency but generates **no** `POST /user/consent` call. `ConsentLogRequest` must never be constructed with `consentType: "ai_disclaimer"` or any other value not in the above list. A backend receiving an unknown consent type will either silently accept it (creating an orphaned record) or reject it (causing an unhandled error); both outcomes are incorrect.
- Valid `consentType` values are `"privacy_policy"` and `"tos"` only. The `"ai_disclaimer"` value is never logged as a consent event. `ComplianceDto.kt` (frontend) and `compliance_dto.go` + `log_consent_usecase.go` (backend) both enforce this allowlist and reject unknown values with `HTTP 400 invalid_consent_type`.
- Idempotent: same `userID + type + version` returns the existing record (safe on reinstall).
- `consent_log` table is insert-only on the backend (`REVOKE UPDATE, DELETE ON consent_log`).
- `PreferenceStore.setConsentAccepted(true)` gates subsequent splash routing.

### Account Deletion

1. User navigates to Profile → Settings → Delete Account.
2. Confirmation dialog displays the **30-day grace period notice** before proceeding.
3. `DeleteAccountUseCase` calls `DELETE /user/account`.
4. On `204 No Content`:
   - `appDatabase.clearAll()` — full wipe of all tables including all Diary tables.
   - `tokenStore.clearAll()` — clears all tokens.
   - FCM token invalidation.
   - Navigate to Login screen.
   - **Error handling:** `appDatabase.clearAll()` and `tokenStore.clearAll()` must each be wrapped in `try/catch`. If `clearAll()` throws (e.g. SQLite lock), log the exception as a non-fatal to Crashlytics and proceed — do not abort the logout navigation. A partial local-data wipe on a server-confirmed deletion is preferable to leaving the user stranded on the deletion screen. The `204` from the server is the authoritative signal; the account is already deleted server-side regardless of local cleanup success.
5. Backend soft-deletes the user row (`deleted_at = NOW()`), revokes all active JTIs, fires `pg NOTIFY account_deleted` — the intelligence service receives this event and **immediately** purges all shop and invoice records for the user (`DELETE FROM shops WHERE user_id = $userID; DELETE FROM invoices WHERE user_id = $userID`). The intelligence listener is idempotent; a duplicate event is safe.
6. Hard-delete of the core user row (and all remaining data not already purged) occurs after the **30-day grace period** via the daily `hard_delete_job.go` cron. During this window, the user's `users` row is soft-deleted (`deleted_at IS NOT NULL`) and all JTIs are revoked — the user cannot authenticate. Shop and invoice data in the intelligence schema is purged immediately in step 5; only the core `users` row and associated auth records are subject to the 30-day delay.

### Logout Data Clearing

`SessionManager.SessionEvent.LoggedOut` (triggered by token expiry, manual sign-out, or 401 cascade) → `authRepository.clearSessionData()` which calls `appDatabase.clearSessionData()`:
- **Clears:** `RateEntity`, `HomeEntity`, `AlertEntity`, `DesignEntity`.
- **MUST NOT clear:** `BillEntity`, `LedgerEntryEntity`, `CustomerEntity` (Diary tables). A jeweller force-logged out by an expired refresh token must not lose their transaction history.

### Data Retention (Backend)

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

## 14. Analytics & Observability Events

All events fire via Firebase Analytics. No PII in event parameters.

| Event | Required Parameters | Trigger |
|---|---|---|
| `rate_viewed` | `cityId: String`, `source: String` — valid values: `"gemini"` (current sole source); future sources (e.g. `"mcx"`, `"manual_override"`) extend this set; **always derive `source` from `rate.source` enum name, never hardcode `"gemini"` as a string literal** | User lands on `RatesDashboardScreen` |
| `alert_created` | `metal: String`, `direction: String` | `createAlert()` succeeds |
| `catalog_searched` | `region: String`, `resultCount: Int` | Search query completes (including empty results; `resultCount` may be 0) |
| `image_search_used` | `region: String` | Image search request submitted (when enabled) |
| `calculator_used` | `metalType: String`, `mode: String` — valid values: `"BUY"`, `"SELL"`; **derive `mode` from `CalculatorMode.name`, never a raw string literal** | Calculator result computed (debounced — only when `metalValue > 0.0` and input stable > 500ms) |
| `bill_generated` | `paymentMode: String` | Invoice generation succeeds |
| `diary_entry_added` | `entryType: String` — valid values: `LEND`, `BORROW`, `PAYMENT`, `RECEIPT`; **derive from `LedgerEntryType.name`, never a raw string literal**; any other value is a bug | Ledger entry or customer created |
| `subscription_flow_started` | _(none)_ | User taps subscription CTA |
| `subscription_verified` | _(none)_ | `POST /billing/verify` returns success |

`screen_view` events are fired automatically by Firebase Analytics. HTTP traces per endpoint and a `cold_start_first_frame` custom trace (target: <80ms) are instrumented via Firebase Performance.

---

## 15. Non-Functional Requirements

| NFR | Target |
|---|---|
| Cold start — first Compose frame from Room | < 80ms |
| Cold start — first meaningful UI (budget) | < 400ms |
| WebSocket connect time | 1–2 seconds |
| Shimmer timeout (first-install no-cache fallback) | Exactly 2,000 ms (hard-coded `delay(2_000)` in `HomeViewModel`; not adaptive — see `HomeViewModel.kt`) |
| BFF `/bff/home` response (cache warm) | < 1,500ms |
| Minimum supported Android SDK | API 24 (Android 7.0 Nougat) |
| Peak concurrent DAU | 10,000 |
| Peak concurrent WebSocket connections | 600–900 |
| REST API peak RPS | ~80–120 RPS |
| Backend uptime target | 99.5% |
| Infra cost ceiling | ~$80/month |
| Pre-launch load test gate | p95 < 500ms, p99 < 2000ms, error rate < 0.1% at 1,200 concurrent users / 750 WS connections (15-min k6 run against staging) |
| PDF invoice size | < 500 KB — enforced by: (a) banner resized to max 1200×400 px at upload-confirm time (backend `confirm_banner_upload_usecase.go`); (b) PDF builder (`invoice_pdf_builder.go`) further resizes banner to max 600×160 px and JPEG-compresses at quality 80 before embedding; raw PNG/WebP from CDN must not be embedded directly. |
| Room migration policy | Explicit `Migration` objects only; `fallbackToDestructiveMigration()` is permanently banned |
| Redis HA | 3-node Redis Sentinel (primary + replica + tie-breaker); required launch gate |

---

## 16. Out-of-Scope / Future Scope

| Item | Notes |
|---|---|
| iOS support | No APNS, no App Store IAP, no Swift code. `push_notification_client.go` is the sole integration point to extend. `APPLE_BUNDLE_ID` / `APPLE_SHARED_SECRET` env vars are documented but not provisioned. |
| Web dashboard | No browser interface in v1 or near-term. |
| B2C retail (consumer gold lookup) | MahaSwarna is a trade tool; consumer-facing price lookup is out of scope. |
| Multi-region backend | Single-VPS Docker Compose. K8s migration documented for ~50k DAU (pricing first), full migration at ~100k DAU or when any service needs >2 replicas. |
| Catalog image search backend | `POST /catalog/image-search` endpoint not yet implemented. Client-side code exists but is gated behind `killSwitchImageSearch = true`. Requires backend intelligence service delivery before enabling. |
| Diary PDF export | Diary entries can be used to regenerate individual invoices. A bulk Diary-to-PDF export format is a TBD. |
| Kafka / external message queue | pg `LISTEN/NOTIFY` is sufficient through ~100k DAU. Kafka migration documented as Step 2 in the upgrade path. |
| Alert in-place editing | No `PUT /alerts/:id` endpoint in v1. Deliberate omission. Must be raised as a new OQ if required. |
| City list dynamic fetch | The 61-city list is a compile-time constant in v1. A `GET /v1/cities` endpoint and dynamic city picker are deferred to a future release. |

---

## 17. Open Questions

| # | Question | Owner | Status |
|---|---|---|---|
| OQ-1 | What is the exact threshold for activating MSG91 fallback in `both` mode? Is it any Firebase SDK error, or only `FirebaseNetworkException`? The architecture documents `FirebaseNetworkException` as the client-side trigger; the server-side silent fallback applies to Firebase Admin SDK infrastructure errors only (not credential failures). The boundary is documented in §5 but should be confirmed by the backend team. | PM + Backend | Open |
| OQ-2 | 30-day account deletion grace period: is this a soft requirement (can be shortened for user request) or a hard compliance requirement? Does the backend need to support early hard-deletion upon explicit user confirmation? | PM + Legal | Open |
| OQ-3 | Diary export format beyond individual invoice PDF regeneration. If a bulk export feature is added, should it export as a single PDF, CSV, or both? What is the retention expectation if exported? | PM | Open |
| OQ-4 | AI quota UI: `AiQuotaInterceptor` reads `X-Ai-Quota-*` headers and exposes `AiQuotaState` (used, limit, resetAt). Which screens should surface the quota indicator, and what happens when `isExhausted == true` — does image search silently disable, or show an explicit quota-exhausted state? | PM + Design | Open |
| OQ-5 | Rate sanity threshold: `FlagsDto` includes `params: Map<String, Double>` containing `rate_sanity_threshold_pct`. How should the client handle a rate update that exceeds the sanity threshold? **RESOLVED:** The Android client does NOT perform any client-side sanity filtering. The threshold is enforced server-side by `rate_quality_watchdog.go`. The client receives the outcome as `stale:true` and shows `StaleRateBanner` — that is the complete client response. The param is parsed and stored in `FeatureFlags.kt` for completeness but is not acted on. | PM + Backend | **Resolved** |
| OQ-6 | `BillPrintScreen` rate unavailability: when `rate.isStale == true` at the time of invoice generation (not just at screen entry), should the bill-generation button be disabled, or should it proceed with a warning? **RESOLVED (v1):** Button is NOT disabled. The post-generation snackbar (`rateSource == "stale"` → "Invoice uses a stale rate — verify before sharing") is the user-facing mechanism. Pre-generation gating requires reactive isStale tracking across screens (the nav arg does not update when WS disconnects) and is deferred to v2. | PM + Design | **Resolved** |
| OQ-7 | Pending bill queue recovery UX: `DiaryViewModel.init()` retries failed `saveBillUseCase` entries silently. Should repeated failure (e.g. after 3 retries) surface a persistent banner or a push notification? **RESOLVED:** `DiaryViewModel.init()` surfaces a "Some bills failed to save" persistent banner on repeated failure (≥ 3 retries). No push notification. | PM + Design | **Resolved** |
| OQ-8 | Kill-switch pre-condition gate: when `killSwitchWs` is activated at full DAU (~1,200 concurrent users), the backend runbook requires raising the FREE-tier BFF rate limit before flipping the switch to avoid a 40 RPS thundering herd. Is this a manual ops step only, or should it be encoded as an automated pre-condition check (e.g. a script that updates the flag and raises the rate limit atomically)? **RESOLVED:** The sequence is encoded in `scripts/activate_ws_killswitch.sh`: STEP 1 — raise `rate_limit_bff_free_rpm` to 60; STEP 2 — sleep 5s for Redis flag cache to refresh; STEP 3 — flip `kill_switch_ws`. This script is the canonical activation mechanism for both manual ops and the automated 3-consecutive-failure path in `generate_ai_rates_usecase.go`. An additional manual pre-check is also required: verify that the deployed APK version on Play Console includes the `±5s jitter` polling implementation (`HomeScreen.kt`) before activating at full DAU. If APK version cannot be confirmed, treat the load as unspread and exercise extra caution. | PM + Backend + Ops | **Resolved** |

