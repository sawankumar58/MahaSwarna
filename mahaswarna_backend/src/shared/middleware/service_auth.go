// Package middleware contains shared HTTP middleware for MahaSwarna microservices.
package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mahaswarna/shared"
	"github.com/mahaswarna/shared/types"
)

// ServiceAuth validates the X-Service-Token + X-Service-Timestamp HMAC-SHA256
// header pair on internal service-to-service routes.
//
// Error format uses the canonical types.Fail envelope (ok=false, error.code, error.message).
// HTTP status is 401 Unauthorized for all failures (token is missing or forged — not a permissions issue).
//
// Caller side (generating the headers):
//
//	token, timestamp := shared.ServiceTokenHeader()
//	req.Header.Set("X-Service-Token", token)
//	req.Header.Set("X-Service-Timestamp", timestamp)
//
// Usage in router.go:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(sharedmiddleware.ServiceAuth)
//	    r.Get("/internal/...", handler.InternalEndpoint)
//	})
func ServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Service-Token")
		tsStr := r.Header.Get("X-Service-Timestamp")

		if token == "" || tsStr == "" {
			writeJSON(w, http.StatusUnauthorized,
				types.Fail[struct{}]("unauthorized", "missing service token"))
			return
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || !shared.VerifyServiceToken(token, ts) {
			writeJSON(w, http.StatusUnauthorized,
				types.Fail[struct{}]("unauthorized", "invalid service token"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck // best-effort response write
}
