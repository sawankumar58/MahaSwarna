package middleware

import (
	"net/http"
	"strconv"

	"github.com/mahaswarna/shared"
)

// ServiceAuth validates the X-Service-Token + X-Service-Timestamp HMAC-SHA256
// header pair on internal service-to-service routes.
//
// This replaces the plain-secret ServiceAuth(secret string) that was in
// middleware.go. That version compared a static token with no replay protection;
// this version matches the pattern used by core and pricing services:
//
//   - X-Service-Timestamp must be within ±30 s of now (replay window).
//   - X-Service-Token must equal HMAC-SHA256(timestamp, INTERNAL_JWT_SECRET).
//
// # Caller side (generating the headers)
//
//	token, timestamp := shared.ServiceTokenHeader()
//	req.Header.Set("X-Service-Token", token)
//	req.Header.Set("X-Service-Timestamp", timestamp)
//
// # Usage in router.go
//
//	r.Group(func(r chi.Router) {
//	    r.Use(mw.ServiceAuth)
//	    r.Get("/internal/...", handler.InternalEndpoint)
//	})
func ServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Service-Token")
		tsStr := r.Header.Get("X-Service-Timestamp")

		if token == "" || tsStr == "" {
			http.Error(w, `{"error":"missing service token"}`, http.StatusForbidden)
			return
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || !shared.VerifyServiceToken(token, ts) {
			http.Error(w, `{"error":"invalid service token"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
