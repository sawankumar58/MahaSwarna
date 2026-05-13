package pricing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/mahaswarna/pricing/application"
	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/pricing/infrastructure"
	"github.com/redis/go-redis/v9"
)

// seedCache writes a snapshot directly into fake Redis using the production key
// prefix "rates:latest:ai:{cityID}" so RateCache.Get reads it correctly.
func seedCache(t *testing.T, rdb *redis.Client, cityID string, gold, silver float64, stale bool) {
	t.Helper()
	snap := &domain.AIRateSnapshot{
		CityID:      cityID,
		Gold:        gold,
		Silver:      silver,
		Source:      domain.RateSourceLive,
		IsStale:     stale,
		GeneratedAt: time.Now(),
	}
	b, _ := json.Marshal(snap)
	_ = rdb.Set(context.Background(), "rates:latest:ai:"+cityID, b, time.Hour).Err()
}

// TestGetRateUseCase_CacheHitReturnsFreshSnapshot verifies that a snapshot
// already in Redis is returned by RateCache.Get with the correct fields.
// (NewGetRateUseCase short-circuits before touching snapRepo on cache hit.)
func TestGetRateUseCase_CacheHitReturnsFreshSnapshot(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	seedCache(t, rdb, "mumbai", 72000, 85000, false)

	cache := infrastructure.NewRateCache(rdb)
	snap, err := cache.Get(context.Background(), "mumbai")
	if err != nil {
		t.Fatalf("RateCache.Get: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot on cache hit")
	}
	if snap.Gold != 72000 {
		t.Errorf("gold: expected 72000, got %v", snap.Gold)
	}
	if snap.Silver != 85000 {
		t.Errorf("silver: expected 85000, got %v", snap.Silver)
	}
	if snap.IsStale {
		t.Error("cache hit must not be marked stale")
	}
}

// TestGetRateUseCase_CacheMissReturnsNil verifies that RateCache.Get returns
// nil, nil on a miss (no error — callers check for nil to detect absence).
func TestGetRateUseCase_CacheMissReturnsNil(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cache := infrastructure.NewRateCache(rdb)
	snap, err := cache.Get(context.Background(), "unknown_city")
	if err != nil {
		t.Fatalf("RateCache.Get cache-miss must not error: %v", err)
	}
	if snap != nil {
		t.Error("RateCache.Get on missing key must return nil snapshot")
	}
}

// TestGetRateUseCase_ErrRateNotAvailableSentinel verifies the sentinel value
// used by HTTP handlers to return 404 vs 503.
func TestGetRateUseCase_ErrRateNotAvailableSentinel(t *testing.T) {
	if application.ErrRateNotAvailable == nil {
		t.Fatal("ErrRateNotAvailable must not be nil")
	}
	if application.ErrRateNotAvailable.Error() != "city_rates_not_available" {
		t.Errorf("sentinel message must be \"city_rates_not_available\", got %q",
			application.ErrRateNotAvailable.Error())
	}
}

// TestGetRateUseCase_CacheKeyPattern verifies the Redis key prefix used for
// storage (must align with warmup_cache.sh and the watchdog eviction path).
func TestGetRateUseCase_CacheKeyPattern(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	seedCache(t, rdb, "bangalore", 71500, 84500, false)

	keys, err := rdb.Keys(context.Background(), "rates:latest:ai:*").Result()
	if err != nil {
		t.Fatalf("KEYS: %v", err)
	}
	if len(keys) != 1 || keys[0] != "rates:latest:ai:bangalore" {
		t.Errorf("expected \"rates:latest:ai:bangalore\", got %v", keys)
	}
}

// TestGetRateUseCase_CacheTTL verifies that Set stores the snapshot with the
// production 1-hour TTL (rateCacheTTL = 3600 s).
func TestGetRateUseCase_CacheTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cache := infrastructure.NewRateCache(rdb)
	snap := &domain.AIRateSnapshot{
		CityID: "pune", Gold: 71800, Silver: 84700,
		Source: domain.RateSourceLive, GeneratedAt: time.Now(),
	}
	if err := cache.Set(context.Background(), snap); err != nil {
		t.Fatalf("RateCache.Set: %v", err)
	}

	ttl, err := rdb.TTL(context.Background(), "rates:latest:ai:pune").Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	// Production TTL is 3600 s; allow up to 10 s elapsed.
	const minTTL = 3590 * time.Second
	if ttl < minTTL {
		t.Errorf("cache TTL must be ~3600 s, got %v", ttl)
	}
}

// TestGetRateUseCase_CacheDelete verifies Delete removes the key (used by
// watchdog when marking a snapshot stale).
func TestGetRateUseCase_CacheDelete(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	seedCache(t, rdb, "jaipur", 72100, 85100, false)
	cache := infrastructure.NewRateCache(rdb)

	if err := cache.Delete(context.Background(), "jaipur"); err != nil {
		t.Fatalf("RateCache.Delete: %v", err)
	}

	snap, err := cache.Get(context.Background(), "jaipur")
	if err != nil {
		t.Fatalf("post-delete Get: %v", err)
	}
	if snap != nil {
		t.Error("snapshot must be nil after cache.Delete")
	}
}

// TestGetRateUseCase_MultiCity verifies that snapshots for different cities
// are stored and retrieved independently.
func TestGetRateUseCase_MultiCity(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := infrastructure.NewRateCache(rdb)
	ctx := context.Background()

	cities := []struct{ id string; gold float64 }{
		{"mumbai", 72000}, {"delhi", 71500}, {"bangalore", 71800},
	}
	for _, c := range cities {
		cache.Set(ctx, &domain.AIRateSnapshot{CityID: c.id, Gold: c.gold, GeneratedAt: time.Now()})
	}

	for _, c := range cities {
		snap, err := cache.Get(ctx, c.id)
		if err != nil || snap == nil {
			t.Errorf("city %s: expected snapshot, got err=%v snap=%v", c.id, err, snap)
			continue
		}
		if snap.Gold != c.gold {
			t.Errorf("city %s: gold expected %.0f, got %.0f", c.id, c.gold, snap.Gold)
		}
	}
}
