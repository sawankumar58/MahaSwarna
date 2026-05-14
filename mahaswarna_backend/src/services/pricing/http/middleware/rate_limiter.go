package middleware

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	sharedmw "github.com/mahaswarna/shared/middleware"
)

// RateLimiter returns a per-IP rate limiter middleware for the pricing service.
// Delegates to the canonical shared implementation with fail-open on Redis error.
// Uses X-Forwarded-For to get the real client IP behind the gateway proxy.
func RateLimiter(rdb *redis.Client, rpm int) func(http.Handler) http.Handler {
	return sharedmw.RateLimiter(rdb, sharedmw.RateLimiterConfig{
		KeyPrefix: "rl:pricing",
		RPM:       rpm,
		PerUser:   false,
	})
}
