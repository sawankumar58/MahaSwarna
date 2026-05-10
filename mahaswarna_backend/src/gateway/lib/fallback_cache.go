package lib

import (
	"context"
	"net/http"
	"time"

	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

// FallbackCache stores and retrieves serialised HTTP response bodies in Redis.
// It is used by ResilientProxy and the BFF aggregator to serve stale data when
// an upstream is down.
//
// Keys have a TTL equal to staleTTL; the circuit breaker's open state is
// expected to be shorter than staleTTL so stale content is available when needed.
type FallbackCache struct {
	rdb      *redis.Client
	staleTTL time.Duration
}

// NewFallbackCache creates a FallbackCache with the given stale TTL.
func NewFallbackCache(rdb *redis.Client, staleTTL time.Duration) *FallbackCache {
	return &FallbackCache{rdb: rdb, staleTTL: staleTTL}
}

// Store saves body under key with the configured TTL.
// Called asynchronously after a successful upstream response.
func (c *FallbackCache) Store(ctx context.Context, key string, body []byte) {
	if err := c.rdb.Set(ctx, key, body, c.staleTTL).Err(); err != nil {
		shared.Logger.Warn("fallback cache: store failed", "key", key, "err", err)
	}
}

// Get retrieves raw bytes stored under key.
// Returns (nil, redis.Nil) if the key does not exist.
func (c *FallbackCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.rdb.Get(ctx, key).Bytes()
}

// ServeStale attempts to write the cached body to w.
// Returns true if a stale response was found and written, false otherwise.
func (c *FallbackCache) ServeStale(ctx context.Context, key string, w http.ResponseWriter) bool {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil || len(data) == 0 {
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "STALE")
	w.WriteHeader(http.StatusOK)
	_, werr := w.Write(data)
	if werr != nil {
		shared.Logger.Warn("fallback cache: write error", "key", key, "err", werr)
	}
	return true
}
