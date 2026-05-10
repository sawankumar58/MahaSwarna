package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRPM    = 120             // requests per minute per IP on pricing endpoints
	rateLimitWindow = time.Minute
)

// RateLimiter returns a middleware that limits requests per IP per minute.
// Uses Redis INCR + EXPIRE for atomic counting.
func RateLimiter(rdb *redis.Client, rpm int) func(http.Handler) http.Handler {
	if rpm <= 0 {
		rpm = defaultRPM
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			key := "rl:pricing:" + ip

			n, err := rdb.Incr(context.Background(), key).Result()
			if err != nil {
				slog.Warn("rate limiter redis error", "err", err)
				next.ServeHTTP(w, r) // fail open
				return
			}
			if n == 1 {
				rdb.Expire(context.Background(), key, rateLimitWindow)
			}
			if n > int64(rpm) {
				w.Header().Set("Retry-After", "60")
				http.Error(w,
					`{"ok":false,"error":{"code":"rate_limited","message":"too many requests"}}`,
					http.StatusTooManyRequests,
				)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func realIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
