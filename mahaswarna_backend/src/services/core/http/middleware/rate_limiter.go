package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func RateLimiter(rdb *redis.Client, rpm int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := fmt.Sprintf("rl:%s", r.RemoteAddr)
			n, _ := rdb.Incr(r.Context(), key).Result()
			if n == 1 { rdb.Expire(r.Context(), key, time.Minute) }
			if int(n) > rpm {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"too_many_requests"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
