package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	mw "github.com/mahaswarna/core/http/middleware"
	"github.com/redis/go-redis/v9"
)

func NewRouter(
	auth *AuthHandler, compliance *ComplianceHandler, billing *BillingHandler,
	alerts *AlertsHandler, deviceToken *DeviceTokenHandler, flags *FlagsHandler,
	internal *InternalHandler, rdb *redis.Client,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RealIP, chimw.RequestID, chimw.Recoverer)

	// Public (no JWT).
	r.Post("/auth/send-otp", auth.SendOTP)
	r.Post("/auth/login",    auth.Login)
	r.Post("/auth/register", auth.Login)
	r.Post("/auth/refresh",  auth.Refresh)

	// JWT-protected.
	r.Group(func(r chi.Router) {
		r.Use(mw.JWTAuth)
		r.Post("/auth/logout",              auth.Logout)
		r.Delete("/user/account",           compliance.DeleteAccount)
		r.Post("/user/consent",             compliance.LogConsent)
		r.Post("/billing/verify",           billing.VerifyReceipt)
		r.Post("/billing/restore",          billing.RestoreSubscription)
		r.Get("/alerts",                    alerts.ListAlerts)
		r.Post("/alerts",                   alerts.CreateAlert)
		r.Delete("/alerts/{id}",            alerts.DeleteAlert)
		r.Post("/engagement/device-token",  deviceToken.RegisterToken)
		r.Get("/flags/public",              flags.GetPublicFlags)
	})

	// Internal — service-to-service (X-Service-Token only).
	r.Group(func(r chi.Router) {
		r.Use(mw.ServiceAuth)
		r.Get("/internal/subscriptions/active", internal.GetActiveSubscriptions)
	})

	// Health probes (see observability/health.go contract).
	r.Get("/health",       func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ready"}) })

	return r
}
