package middleware

import (
	"net/http"

	sharedmw "github.com/mahaswarna/shared/middleware"
)

// ServiceAuth validates X-Service-Token + X-Service-Timestamp on internal routes.
// Delegates to the canonical shared implementation to ensure consistent error
// envelope format and HTTP status codes across all services.
func ServiceAuth(next http.Handler) http.Handler {
	return sharedmw.ServiceAuth(next)
}
