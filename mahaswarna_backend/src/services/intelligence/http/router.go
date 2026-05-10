package handler

import (
	"crypto/rsa"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	mw "github.com/mahaswarna/intelligence/http/middleware"
)

// NewRouter assembles the chi router for the intelligence service.
//
// Route structure:
//
//	GET  /health                               — liveness probe (no auth)
//	POST /v1/shops                             — register shop (JWT + premium guard in usecase)
//	GET  /v1/shops                             — get authenticated user's shops as array (JWT)
//	POST /v1/shops/{id}/banner                 — get presigned S3 PUT URL (JWT + premium)
//	POST /v1/shops/{id}/banner/confirm         — confirm upload + moderation (JWT)
//	GET  /v1/catalog/search                    — full-text design search (JWT)
//	GET  /v1/catalog/designs/{id}              — single design + view track (JWT)
//	GET  /v1/catalog/recommendations           — trending designs (JWT)
//	POST /v1/shops/{id}/invoices               — generate PDF invoice (JWT + premium)
//	GET  /v1/shops/{shopId}/invoices           — list invoice history (JWT, owner only)
func NewRouter(
	pubKey *rsa.PublicKey,
	rdb *redis.Client,
	shops *ShopHandler,
	catalog *CatalogHandler,
	invoices *InvoiceHandler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	// Liveness probe — no auth.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(mw.JWTAuth(pubKey))

		// Shops.
		r.Post("/v1/shops", shops.RegisterShop)
		r.Get("/v1/shops", shops.GetShop)
		r.Post("/v1/shops/{id}/banner", shops.GetBannerUploadURL)
		r.Post("/v1/shops/{id}/banner/confirm", shops.ConfirmBannerUpload)

		// Catalog — rate-limited at 120 req/min per user.
		r.Group(func(r chi.Router) {
			r.Use(mw.RateLimiter(rdb, "catalog", 120))
			r.Get("/v1/catalog/search", catalog.Search)
			r.Get("/v1/catalog/designs/{id}", catalog.GetDesign)
			r.Get("/v1/catalog/recommendations", catalog.Recommend)
		})

		// Invoices — rate-limited at 10 req/min per user (daily limit enforced in usecase).
		// POST /v1/shops/{id}/invoices   — spec uses path param {id} for the shop.
		// GET  /v1/shops/{shopId}/invoices — list history; {shopId} kept distinct for clarity.
		r.Group(func(r chi.Router) {
			r.Use(mw.RateLimiter(rdb, "invoice", 10))
			r.Post("/v1/shops/{id}/invoices", invoices.GenerateInvoice)
			r.Get("/v1/shops/{shopId}/invoices", invoices.ListInvoices)
		})
	})

	return r
}
