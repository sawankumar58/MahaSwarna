package middleware

import (
	"net/http"
	"strconv"

	"github.com/mahaswarna/shared"
)

// ServiceAuth validates the X-Service-Token HMAC-SHA256 header on internal routes.
// Used to protect endpoints that are only called by other services (gateway BFF, core).
func ServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Service-Token")
		tsStr := r.Header.Get("X-Service-Timestamp")

		if token == "" || tsStr == "" {
			http.Error(w, `{"ok":false,"error":{"code":"unauthorized","message":"missing service token"}}`,
				http.StatusUnauthorized)
			return
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || !shared.VerifyServiceToken(token, ts) {
			http.Error(w, `{"ok":false,"error":{"code":"unauthorized","message":"invalid service token"}}`,
				http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
