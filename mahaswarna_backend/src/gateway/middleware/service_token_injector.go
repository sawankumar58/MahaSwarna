package middleware

import (
	"net/http"

	"github.com/mahaswarna/shared"
)

// ServiceTokenInjector generates and injects the X-Service-Token and
// X-Service-Timestamp headers required for inter-service authentication.
//
// Upstream services (core, pricing, intelligence) call shared.VerifyServiceToken
// to validate these headers. The token is HMAC-SHA256(timestamp, INTERNAL_JWT_SECRET)
// and is only valid within ±30 seconds of the timestamp to prevent replay attacks.
//
// Must be applied AFTER JWTPreValidator so that user headers are already set.
func ServiceTokenInjector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, timestamp := shared.ServiceTokenHeader()
		r.Header.Set("X-Service-Token", token)
		r.Header.Set("X-Service-Timestamp", timestamp)
		next.ServeHTTP(w, r)
	})
}
