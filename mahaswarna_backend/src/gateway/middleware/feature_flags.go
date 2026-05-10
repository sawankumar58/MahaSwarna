package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mahaswarna/shared"
	contractshttp "github.com/mahaswarna/contracts/http"
	"github.com/redis/go-redis/v9"
)

const (
	flagsCacheKey = "cache:flags:current"
	flagsCacheTTL = 5 * time.Minute
)

type ctxKeyFlags struct{}

// FeatureFlags loads the current feature flag state from Redis (written by the
// core service whenever flags change) and injects it into the request context.
//
// If Redis is unavailable or the key is missing the middleware proceeds with
// an empty flags object so the gateway degrades gracefully rather than failing.
//
// The flag state is refreshed every 5 minutes by core's flag_updated event
// pipeline; the gateway does NOT poll core on every request.
func FeatureFlags(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flags := loadFlags(r.Context(), rdb)
			ctx := context.WithValue(r.Context(), ctxKeyFlags{}, flags)

			// Propagate kill-switch state to upstreams as a header so they can
			// short-circuit expensive operations without Redis calls.
			if flags != nil {
				if ws, ok := flags.KillSwitch["ws"]; ok && ws {
					r.Header.Set("X-Kill-WS", "true")
				}
				if ai, ok := flags.KillSwitch["ai"]; ok && ai {
					r.Header.Set("X-Kill-AI", "true")
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FlagsFromCtx returns the feature flags from the context, or nil if absent.
func FlagsFromCtx(ctx context.Context) *contractshttp.FeatureFlagsResponse {
	v, _ := ctx.Value(ctxKeyFlags{}).(*contractshttp.FeatureFlagsResponse)
	return v
}

// IsKillSwitchActive checks if a named kill switch is currently active.
// name examples: "ws", "ai", "image_search"
func IsKillSwitchActive(ctx context.Context, name string) bool {
	flags := FlagsFromCtx(ctx)
	if flags == nil {
		return false
	}
	return flags.KillSwitch[name]
}

func loadFlags(ctx context.Context, rdb *redis.Client) *contractshttp.FeatureFlagsResponse {
	data, err := rdb.Get(ctx, flagsCacheKey).Bytes()
	if err != nil {
		if err != redis.Nil {
			shared.Logger.Warn("feature flags: redis error, proceeding without flags", "err", err)
		}
		return nil
	}

	var flags contractshttp.FeatureFlagsResponse
	if err := json.Unmarshal(data, &flags); err != nil {
		shared.Logger.Warn("feature flags: unmarshal error", "err", err)
		return nil
	}
	return &flags
}

// Compile-time: flagsCacheTTL is intentional (used by cache setters in BFF aggregator).
var _ = flagsCacheTTL
