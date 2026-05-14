package middleware

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	sharedmw "github.com/mahaswarna/shared/middleware"
)

// RateLimiter returns a per-user rate limiter middleware for the intelligence service.
// Requires JWTAuth middleware to run earlier in the chain.
// Delegates to the canonical shared implementation with fail-open on Redis error.
func RateLimiter(rdb *redis.Client, endpoint string, limit int) func(http.Handler) http.Handler {
	return sharedmw.RateLimiter(rdb, sharedmw.RateLimiterConfig{
		KeyPrefix:     "rl:intel:" + endpoint,
		RPM:           limit,
		PerUser:       true,
		UserIDFromCtx: UserIDFromCtx,
	})
}
