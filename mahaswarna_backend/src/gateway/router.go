package main

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/mahaswarna/gateway/bff"
	"github.com/mahaswarna/gateway/lib"
	"github.com/mahaswarna/gateway/middleware"
	"github.com/mahaswarna/shared"
)

// upstreamURL resolves an upstream base URL from the environment.
func upstreamURL(env string) string { return os.Getenv(env) }

func buildRouter(rdb *redis.Client) http.Handler {
	r := chi.NewRouter()

	// ── Upstream proxy targets ─────────────────────────────────────────────
	coreURL         := upstreamURL("CORE_BASE_URL")         // http://core:4001
	pricingURL      := upstreamURL("PRICING_BASE_URL")      // http://pricing:4002
	intelligenceURL := upstreamURL("INTELLIGENCE_BASE_URL") // http://intelligence:4003

	// ── Circuit breakers (one per upstream) ───────────────────────────────
	coreBreaker        := lib.NewBreaker("core")
	pricingBreaker     := lib.NewBreaker("pricing")
	intelligenceBreaker := lib.NewBreaker("intelligence")

	// ── Resilient proxies ──────────────────────────────────────────────────
	coreProxy        := lib.NewResilientProxy(coreURL, coreBreaker, rdb)
	pricingProxy     := lib.NewResilientProxy(pricingURL, pricingBreaker, rdb)
	intelligenceProxy := lib.NewResilientProxy(intelligenceURL, intelligenceBreaker, rdb)

	// ── BFF aggregator ─────────────────────────────────────────────────────
	homeAgg := bff.NewHomeAggregator(
		coreURL, pricingURL, intelligenceURL,
		coreBreaker, pricingBreaker, intelligenceBreaker,
		rdb,
	)

	// ── Global middleware stack ────────────────────────────────────────────
	// Order matches ARCHITECTURE.md §Gateway middleware chain:
	//   RealIP → RequestID → TraceContext → Recoverer
	//   → APIVersionValidator → VersionValidator → GlobalRateLimiter
	//
	// APIVersionValidator MUST precede JWTPreValidator (blocks deprecated API
	// clients before any token processing). VersionValidator (semver X-App-Version)
	// follows immediately after.
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)           // sets X-Request-ID
	r.Use(middleware.TraceContext)        // propagates trace headers downstream
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.APIVersionValidator) // Accept-Version: 410 deprecated / 400 unsupported
	r.Use(middleware.VersionValidator)    // X-App-Version: 426 if below MIN_APP_VERSION
	r.Use(middleware.GlobalRateLimiter(rdb, shared.DefaultGlobalPolicy))

	// ── Health (no auth, no version checks needed) ─────────────────────────
	r.Get("/health", healthHandler)
	r.Get("/health/ready", readyzHandler(rdb)) // OpenAPI: GET /health/ready

	// ── Auth routes (no JWT required, core upstream) ───────────────────────
	// POST /auth/* are wrapped with per-request idempotency; the idempotency
	// middleware namespaces by remote address when no userID is in context.
	r.Route("/v1/auth", func(r chi.Router) {
		r.Use(middleware.Idempotency(rdb))
		r.Post("/send-otp", coreProxy.Handle)  // POST /v1/auth/send-otp
		r.Post("/login", coreProxy.Handle)     // POST /v1/auth/login
		r.Post("/refresh", coreProxy.Handle)   // POST /v1/auth/refresh
		r.Post("/logout", coreProxy.Handle)    // POST /v1/auth/logout
		r.Post("/register", coreProxy.Handle)  // POST /v1/auth/register (non-Android clients)
	})

	// ── Authenticated routes ───────────────────────────────────────────────
	// Full authenticated middleware chain per ARCHITECTURE.md:
	//   JWTPreValidator → FeatureFlags → ServiceTokenInjector → Idempotency → AbuseDetector
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTPreValidator)
		r.Use(middleware.FeatureFlags(rdb))        // injects flag state into ctx; propagates kill-switches
		r.Use(middleware.ServiceTokenInjector)     // injects X-Service-Token for upstreams
		r.Use(middleware.Idempotency(rdb))         // global idempotency for all authenticated mutations
		r.Use(middleware.AbuseDetector(rdb))       // post-auth; correlates userID in logs

		// BFF — home screen aggregation (tier-rate-limited)
		r.With(middleware.TierRateLimiter(rdb, shared.DefaultBFFPolicy)).
			Get("/v1/bff/home", homeAgg.Handle)

		// ── Rates (pricing service) ────────────────────────────────────────
		// S-1 fix: cityID as URL path segment per OpenAPI spec (GET /rates/{cityID}).
		// The query-param form (?cityId=) is handled internally by the pricing service
		// when it receives forwarded path segments from the gateway.
		r.Route("/v1/rates", func(r chi.Router) {
			r.Get("/{cityID}", pricingProxy.Handle)         // GET /v1/rates/{cityID}
			r.Get("/{cityID}/history", pricingProxy.Handle) // GET /v1/rates/{cityID}/history
			// NOTE: /internal/rates/ai is consumed only by the BFF aggregator via
			// a direct HTTP call to the pricing service (not proxied through this router).
			// It is intentionally absent here to avoid exposing an undocumented public surface.
		})

		// ── Alerts (core service) ──────────────────────────────────────────
		r.Route("/v1/alerts", func(r chi.Router) {
			r.Get("/", coreProxy.Handle)
			r.Post("/", coreProxy.Handle)
			r.Delete("/{alertID}", coreProxy.Handle)
		})

		// ── Billing / IAP (core service) ──────────────────────────────────
		r.Route("/v1/billing", func(r chi.Router) {
			r.Post("/verify", coreProxy.Handle)
			r.Post("/restore", coreProxy.Handle)
		})

		// ── User routes — consent and account management ───────────────────
		r.Post("/v1/user/consent", coreProxy.Handle)   // POST /v1/user/consent
		r.Delete("/v1/user/account", coreProxy.Handle) // DELETE /v1/user/account (compliance deletion)

		// ── Engagement / push notification (core service) ─────────────────
		r.Post("/v1/engagement/device-token", coreProxy.Handle) // POST — register/update FCM token
		// TODO(openapi): DELETE /v1/engagement/device-token/{token} is implemented here
		// but not yet documented in mahaswarna-openapi.yaml. Add before next release.
		r.Delete("/v1/engagement/device-token/{token}", coreProxy.Handle)

		// ── Feature flags — read-only snapshot (core service) ─────────────
		r.Get("/v1/config/feature-flags", coreProxy.Handle)

		// ── Catalog — AI-gated (intelligence service) ─────────────────────
		// M-1 fix: bare GET /v1/catalog/ removed — no OpenAPI counterpart.
		// C-2 / C-1 fix: AIQuotaInterceptor is now a response-side header relay
		// (maps X-Internal-Ai-Quota-* → X-Ai-Quota-*); it no longer takes rdb.
		r.Route("/v1/catalog", func(r chi.Router) {
			r.Use(middleware.AIQuotaInterceptor)
			r.Get("/search", intelligenceProxy.Handle)       // GET /v1/catalog/search
			r.Get("/recommend", intelligenceProxy.Handle)    // GET /v1/catalog/recommend
			r.Get("/designs/{id}", intelligenceProxy.Handle) // GET /v1/catalog/designs/{id}
			// POST /image-search is intentionally absent — gated by killSwitchImageSearch.
			// Re-add in coordinated release once killSwitchImageSearch is set to false.
		})

		// ── Shops / marketplace (intelligence service) ─────────────────────
		// C-2 fix: renamed /v1/shop → /v1/shops (matches OpenAPI plural form).
		// Added: POST (create), {shopID}/banner, {shopID}/banner/confirm,
		//        {shopID}/invoice/generate per OpenAPI spec.
		r.Route("/v1/shops", func(r chi.Router) {
			r.Post("/", intelligenceProxy.Handle)                          // POST  /v1/shops — create shop (PREMIUM only)
			r.Get("/", intelligenceProxy.Handle)                           // GET   /v1/shops — get my shop profile
			r.Put("/", intelligenceProxy.Handle)                           // PUT   /v1/shops — update shop profile
			r.Post("/{shopID}/banner", intelligenceProxy.Handle)           // POST  /v1/shops/{shopID}/banner — get presigned S3 URL
			r.Post("/{shopID}/banner/confirm", intelligenceProxy.Handle)   // POST  /v1/shops/{shopID}/banner/confirm — trigger moderation + resize
			r.Post("/{shopID}/invoice/generate", intelligenceProxy.Handle) // POST  /v1/shops/{shopID}/invoice/generate — GST PDF (ADR-001)
			r.Get("/invoices", intelligenceProxy.Handle)                   // GET   /v1/shops/invoices — list my invoices
		})
	})

	return r
}

// healthHandler returns 200 immediately — used by Docker HEALTHCHECK.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// readyzHandler checks Redis connectivity before reporting ready.
// Path: GET /health/ready (matches OpenAPI spec).
func readyzHandler(rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		if err := rdb.Ping(ctx).Err(); err != nil {
			shared.Logger.Warn("readyz: redis not ready", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"redis_unavailable","message":"Redis is not ready"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}
