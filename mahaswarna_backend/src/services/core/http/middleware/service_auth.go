package middleware

import (
	"net/http"
	"strconv"

	"github.com/mahaswarna/shared"
)

func ServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Service-Token")
		ts, err := strconv.ParseInt(r.Header.Get("X-Service-Timestamp"), 10, 64)
		if err != nil || !shared.VerifyServiceToken(token, ts) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
