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

const (
	// abuseBurstWindow is the time window in which we count burst requests.
	abuseBurstWindow = 10 * time.Second
	// abuseBurstLimit is the max requests in abuseBurstWindow before flagging.
	abuseBurstLimit = 100
	// abuseBlockDuration is how long the IP is blocked after abuse is detected.
	// Exponential: first offence = 60s, second = 120s, third = 240s … capped at 1h.
	abuseBlockBase = 60 * time.Second
	abuseBlockMax  = 1 * time.Hour
)

// AbuseDetector tracks rapid burst traffic per IP and blocks abusive callers
// with exponentially increasing back-off durations. Unlike GlobalRateLimiter
// (which is a steady-state RPM check), AbuseDetector is tuned to catch
// short-burst floods (e.g. DDoS tooling, misconfigured clients).
//
// Redis keys used:
//
//	abuse:burst:{ip}   – sorted-set sliding window (10 s)
//	abuse:block:{ip}   – string key, value = block count; TTL = block duration
//	abuse:count:{ip}   – int; incremented on each block, never expires
func AbuseDetector(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			ctx := r.Context()

			// Check existing block first (fast path).
			if blocked, retryAfter := isBlocked(ctx, rdb, ip); blocked {
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				writeError(w, http.StatusTooManyRequests, "abuse_detected",
					fmt.Sprintf("too many requests; you are blocked for %ds", retryAfter))
				return
			}

			// Sliding-window burst counter.
			burstKey := fmt.Sprintf("abuse:burst:%s", ip)
			nowMs := time.Now().UnixMilli()
			cutoffMs := nowMs - abuseBurstWindow.Milliseconds()

			pipe := rdb.Pipeline()
			pipe.ZRemRangeByScore(ctx, burstKey, "-inf", strconv.FormatInt(cutoffMs, 10))
			pipe.ZAdd(ctx, burstKey, redis.Z{Score: float64(nowMs), Member: fmt.Sprintf("%d", nowMs)})
			pipe.ZCard(ctx, burstKey)
			pipe.Expire(ctx, burstKey, abuseBurstWindow*2)
			cmds, err := pipe.Exec(ctx)
			if err != nil {
				// Redis failure → fail open.
				shared.Logger.Warn("abuse detector redis error, failing open", "ip", ip, "err", err)
				next.ServeHTTP(w, r)
				return
			}

			count := cmds[2].(*redis.IntCmd).Val()
			if count > abuseBurstLimit {
				blockDuration := calculateBlockDuration(ctx, rdb, ip)
				applyBlock(ctx, rdb, ip, blockDuration)

				shared.Logger.Warn("abuse detected, blocking IP",
					"ip", ip,
					"burst_count", count,
					"block_seconds", int(blockDuration.Seconds()),
				)

				retryAfter := int64(blockDuration.Seconds())
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				writeError(w, http.StatusTooManyRequests, "abuse_detected",
					fmt.Sprintf("abusive traffic pattern detected; blocked for %ds", retryAfter))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isBlocked(ctx context.Context, rdb *redis.Client, ip string) (bool, int64) {
	blockKey := fmt.Sprintf("abuse:block:%s", ip)
	ttl, err := rdb.TTL(ctx, blockKey).Result()
	if err != nil || ttl <= 0 {
		return false, 0
	}
	return true, int64(ttl.Seconds()) + 1
}

func calculateBlockDuration(ctx context.Context, rdb *redis.Client, ip string) time.Duration {
	countKey := fmt.Sprintf("abuse:count:%s", ip)
	n, _ := rdb.Get(ctx, countKey).Int64()
	// Exponential back-off: 60s * 2^n, capped at 1h.
	d := abuseBlockBase
	for i := int64(0); i < n; i++ {
		d *= 2
		if d > abuseBlockMax {
			d = abuseBlockMax
			break
		}
	}
	return d
}

func applyBlock(ctx context.Context, rdb *redis.Client, ip string, duration time.Duration) {
	blockKey := fmt.Sprintf("abuse:block:%s", ip)
	countKey := fmt.Sprintf("abuse:count:%s", ip)
	pipe := rdb.Pipeline()
	pipe.Set(ctx, blockKey, "1", duration)
	pipe.Incr(ctx, countKey)
	// count key has no TTL — we want persistent escalation per IP.
	_, _ = pipe.Exec(ctx)
}
