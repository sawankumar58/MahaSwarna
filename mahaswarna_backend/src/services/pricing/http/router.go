package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/mahaswarna/observability"
	pricingmw "github.com/mahaswarna/pricing/http/middleware"
	"github.com/mahaswarna/pricing/ws"
	"github.com/redis/go-redis/v9"
)

// NewRouter builds the chi router for the pricing service.
// WS upgrade lives at GET /ws (handled by ws.Server directly — not chi).
func NewRouter(
	ratesH *RatesHandler,
	wsServer *ws.Server,
	health *observability.HealthChecker,
	rdb *redis.Client,
) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(pricingmw.RateLimiter(rdb, 120))

	// Health — no auth required.
	r.Get("/health", health.LivenessHandler())
	r.Get("/health/ready", health.ReadinessHandler())

	// WebSocket upgrade — JWT auth is enforced inside ws.Server.ServeHTTP.
	r.Get("/ws", wsServer.ServeHTTP)

	// Rate REST endpoints — protected by service auth (called from gateway BFF).
	r.Group(func(r chi.Router) {
		r.Use(pricingmw.ServiceAuth)
		r.Get("/rates/{cityID}", ratesH.GetRate)
		r.Get("/rates/{cityID}/history", ratesH.GetHistory)
	})

	return r
}
