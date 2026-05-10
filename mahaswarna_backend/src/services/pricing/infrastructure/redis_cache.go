package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mahaswarna/pricing/domain"
	"github.com/redis/go-redis/v9"
)

const (
	rateCacheTTL    = 3600 * time.Second // 1 hour — matches AI scheduler cadence
	rateCachePrefix = "rates:latest:ai:"
)

// RateCache wraps Redis for gold/silver rate storage.
// Key pattern: rates:latest:ai:{cityID}
type RateCache struct {
	rdb *redis.Client
}

func NewRateCache(rdb *redis.Client) *RateCache {
	return &RateCache{rdb: rdb}
}

func cacheKey(cityID string) string {
	return rateCachePrefix + cityID
}

// Set stores a snapshot in Redis with a 1-hour TTL.
func (c *RateCache) Set(ctx context.Context, snap *domain.AIRateSnapshot) error {
	b, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("rate cache marshal: %w", err)
	}
	if err := c.rdb.Set(ctx, cacheKey(snap.CityID), b, rateCacheTTL).Err(); err != nil {
		return fmt.Errorf("rate cache set %s: %w", snap.CityID, err)
	}
	return nil
}

// Get returns the cached snapshot for a city, or nil if absent.
func (c *RateCache) Get(ctx context.Context, cityID string) (*domain.AIRateSnapshot, error) {
	b, err := c.rdb.Get(ctx, cacheKey(cityID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("rate cache get %s: %w", cityID, err)
	}
	var snap domain.AIRateSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, fmt.Errorf("rate cache unmarshal %s: %w", cityID, err)
	}
	return &snap, nil
}

// Delete removes the cached rate for a city (used when a stale flag is set).
func (c *RateCache) Delete(ctx context.Context, cityID string) error {
	return c.rdb.Del(ctx, cacheKey(cityID)).Err()
}

// WarmAll bulk-loads all snapshots into Redis. Called by warmup_cache.sh and
// after manual_override inserts.
func (c *RateCache) WarmAll(ctx context.Context, snaps []*domain.AIRateSnapshot) error {
	pipe := c.rdb.Pipeline()
	for _, s := range snaps {
		b, err := json.Marshal(s)
		if err != nil {
			continue
		}
		pipe.Set(ctx, cacheKey(s.CityID), b, rateCacheTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}
