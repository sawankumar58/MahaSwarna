package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRPM      = 120
	rateLimitWindow = time.Minute
)

// RateLimiterConfig holds options for the shared RateLimiter.
type RateLimiterConfig struct {
	// KeyPrefix is prepended to every Redis key. Use a service-specific value
	// (e.g. "rl:core", "rl:pricing", "rl:intel") to avoid cross-service key collisions.
	KeyPrefix string

	// RPM is the maximum requests per minute. Defaults to 120 when zero or negative.
	RPM int

	// PerUser when true uses the authenticated userID from context as the key discriminant.
	// Requires JWTAuth middleware to run before RateLimiter in the chain.
	// When false (default) uses the real client IP address.
	PerUser bool

	// UserIDFromCtx extracts the user UUID from the request context.
	// Required when PerUser == true; ignored otherwise.
	// Inject your service-local UserIDFromCtx function here.
	UserIDFromCtx func(ctx context.Context) (uuid.UUID, bool)
}

// RateLimiter returns a middleware that limits requests per minute.
// Uses Redis INCR + EXPIRE for atomic counting. Fails open on Redis error (logs warning).
//
// Usage — per-IP (core, pricing):
//
//	r.Use(sharedmw.RateLimiter(rdb, sharedmw.RateLimiterConfig{
//	    KeyPrefix: "rl:core",
//	    RPM:       120,
//	}))
//
// Usage — per-user (intelligence):
//
//	r.Use(sharedmw.RateLimiter(rdb, sharedmw.RateLimiterConfig{
//	    KeyPrefix:      "rl:intel:catalog",
//	    RPM:            120,
//	    PerUser:        true,
//	    UserIDFromCtx:  mw.UserIDFromCtx,
//	}))
func RateLimiter(rdb *redis.Client, cfg RateLimiterConfig) func(http.Handler) http.Handler {
	if cfg.RPM <= 0 {
		cfg.RPM = defaultRPM
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var key string
			if cfg.PerUser && cfg.UserIDFromCtx != nil {
				userID, ok := cfg.UserIDFromCtx(r.Context())
				if !ok {
					// Auth middleware hasn't run or user is unauthenticated — skip limiting.
					next.ServeHTTP(w, r)
					return
				}
				key = fmt.Sprintf("%s:%s", cfg.KeyPrefix, userID)
			} else {
				key = fmt.Sprintf("%s:%s", cfg.KeyPrefix, realClientIP(r))
			}

			n, err := rdb.Incr(context.Background(), key).Result()
			if err != nil {
				slog.Warn("rate limiter redis error", "key", key, "err", err)
				next.ServeHTTP(w, r) // fail open
				return
			}
			if n == 1 {
				rdb.Expire(context.Background(), key, rateLimitWindow) //nolint:errcheck
			}
			if n > int64(cfg.RPM) {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, http.StatusTooManyRequests,
					types429())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realClientIP extracts the real client IP, respecting X-Forwarded-For set by trusted proxies.
func realClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

// types429 returns the canonical rate-limit error payload.
func types429() map[string]any {
	return map[string]any{
		"ok":    false,
		"error": map[string]string{"code": "rate_limited", "message": "too many requests"},
	}
}
