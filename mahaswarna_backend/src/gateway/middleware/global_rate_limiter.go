package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

// slidingWindowLua is an atomic Redis Lua script implementing a
// token-bucket-style sliding window rate limiter.
//
// Keys:  KEYS[1] = rate limit bucket key
// Args:  ARGV[1] = window duration in seconds
//
//	ARGV[2] = max requests in that window
//	ARGV[3] = current Unix timestamp (milliseconds)
//
// Returns: {current_count, ttl_ms}
var slidingWindowLua = redis.NewScript(`
local key    = KEYS[1]
local window = tonumber(ARGV[1]) * 1000   -- ms
local limit  = tonumber(ARGV[2])
local now    = tonumber(ARGV[3])
local cutoff = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', cutoff)
local count = redis.call('ZCARD', key)

if count < limit then
  redis.call('ZADD', key, now, now .. '-' .. math.random(1,1000000))
  redis.call('PEXPIRE', key, window)
  return {count + 1, 0}
end

-- Oldest member tells us when the window will free a slot.
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry_after_ms = 0
if #oldest >= 2 then
  retry_after_ms = window - (now - tonumber(oldest[2]))
end
return {count, retry_after_ms}
`)

// GlobalRateLimiter applies a per-IP sliding-window rate limit before any
// JWT parsing, so unauthenticated flood attacks are stopped early.
//
// The policy RPM values are used as the window limit (60-second window).
// Before the user is authenticated we can only use the free-tier RPM;
// post-authentication tier-specific limiting is applied by TierRateLimiter.
func GlobalRateLimiter(rdb *redis.Client, policy shared.RateLimitPolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			key := fmt.Sprintf("rl:global:%s", ip)

			allowed, retryMs := checkLimit(r.Context(), rdb, key, 60, policy.FreeRPM)
			if !allowed {
				retryAfter := retryMs / 1000
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(policy.FreeRPM))
				writeError(w, http.StatusTooManyRequests, shared.ErrTooManyRequests.Error(),
					fmt.Sprintf("rate limit exceeded; retry after %ds", retryAfter))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TierRateLimiter applies per-user sliding-window limits keyed on userID+endpoint.
// Must be used after JWTPreValidator so UserIDFromCtx returns a non-empty string.
func TierRateLimiter(rdb *redis.Client, policy shared.RateLimitPolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromCtx(r.Context())
			if userID == "" {
				// Fallback: treat as unauthenticated; JWTPreValidator should have caught this.
				next.ServeHTTP(w, r)
				return
			}

			tier := TierFromCtx(r.Context())
			limit := limitForTier(tier, policy)
			key := fmt.Sprintf("rl:tier:%s:%s", userID, r.URL.Path)

			allowed, retryMs := checkLimit(r.Context(), rdb, key, 60, limit)
			if !allowed {
				retryAfter := retryMs / 1000
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Tier", tier)
				writeError(w, http.StatusTooManyRequests, shared.ErrTooManyRequests.Error(),
					fmt.Sprintf("rate limit exceeded for %s tier; retry after %ds", tier, retryAfter))
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Tier", tier)
			next.ServeHTTP(w, r)
		})
	}
}

// checkLimit runs the Lua script and returns (allowed, retryAfterMs).
func checkLimit(ctx context.Context, rdb *redis.Client, key string, windowSecs, maxReqs int) (bool, int64) {
	nowMs := time.Now().UnixMilli()
	res, err := slidingWindowLua.Run(ctx, rdb, []string{key},
		strconv.Itoa(windowSecs),
		strconv.Itoa(maxReqs),
		strconv.FormatInt(nowMs, 10),
	).Int64Slice()
	if err != nil || len(res) < 2 {
		// Redis failure → fail open (don't block legitimate traffic).
		shared.Logger.Warn("rate limiter redis error, failing open", "key", key, "err", err)
		return true, 0
	}

	count := res[0]
	retryMs := res[1]
	return count <= int64(maxReqs), retryMs
}

func limitForTier(tier string, policy shared.RateLimitPolicy) int {
	switch tier {
	case "premium":
		return policy.PremiumRPM
	case "admin":
		return policy.AdminRPM
	default:
		return policy.FreeRPM
	}
}
