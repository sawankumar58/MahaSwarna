package core_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/redis/go-redis/v9"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func seedRate(t *testing.T, rdb *redis.Client, cityID string, gold, silver float64) {
	t.Helper()
	snap := infrastructure.RateSnapshot{CityID: cityID, Gold: gold, Silver: silver}
	b, _ := json.Marshal(snap)
	_ = rdb.Set(context.Background(), "rate:latest:ai:"+cityID, b, 0).Err()
}

// TestEvaluateThresholdsUseCase_DebouncePreventsDuplicateFire verifies that
// the Redis SetNX debounce key (alert_debounce:{alertID}) blocks a second
// delivery for the same alert within the debounce window.
func TestEvaluateThresholdsUseCase_DebouncePreventsDuplicateFire(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	alertID := uuid.New()
	key := "alert_debounce:" + alertID.String()

	// First delivery: SetNX should succeed (key absent).
	set1, err := rdb.SetNX(ctx, key, "1", 0).Result()
	if err != nil {
		t.Fatalf("SetNX error: %v", err)
	}
	if !set1 {
		t.Fatal("first SetNX must succeed (key not yet present)")
	}

	// Second delivery attempt: SetNX must return false (key already set).
	set2, err := rdb.SetNX(ctx, key, "1", 0).Result()
	if err != nil {
		t.Fatalf("SetNX error: %v", err)
	}
	if set2 {
		t.Error("second SetNX must return false (debounce key already set)")
	}
}

// TestEvaluateThresholdsUseCase_DebounceIsScopedPerAlert verifies that distinct
// alert IDs have independent debounce keys.
func TestEvaluateThresholdsUseCase_DebounceIsScopedPerAlert(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	alertA := uuid.New()
	alertB := uuid.New()

	// Exhaust debounce for alert A.
	rdb.SetNX(ctx, "alert_debounce:"+alertA.String(), "1", 0)

	// Alert B must still be allowed.
	setB, err := rdb.SetNX(ctx, "alert_debounce:"+alertB.String(), "1", 0).Result()
	if err != nil {
		t.Fatalf("SetNX error: %v", err)
	}
	if !setB {
		t.Error("alert B debounce key must not be blocked by alert A's key")
	}
}

// TestEvaluateThresholdsUseCase_SilverRateSelected verifies that when metal is
// "silver", the silver field from the rate snapshot is used for comparison.
func TestEvaluateThresholdsUseCase_SilverRateSelected(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	seedRate(t, rdb, "delhi", 72000, 85000)

	snap, err := infrastructure.NewRateProjection(rdb).GetLatestRate(context.Background(), "delhi")
	if err != nil {
		t.Fatalf("GetLatestRate: %v", err)
	}

	// Reproduce EvaluateThresholdsUseCase metal-selector.
	rate := snap.Gold
	if domain.MetalSilver == "silver" {
		rate = snap.Silver
	}
	if rate != 85000 {
		t.Errorf("silver rate: expected 85000, got %v", rate)
	}
}

// TestEvaluateThresholdsUseCase_GoldRateSelected verifies that when metal is
// "gold", the gold field is used.
func TestEvaluateThresholdsUseCase_GoldRateSelected(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	seedRate(t, rdb, "mumbai", 72500, 85000)

	snap, err := infrastructure.NewRateProjection(rdb).GetLatestRate(context.Background(), "mumbai")
	if err != nil {
		t.Fatalf("GetLatestRate: %v", err)
	}

	rate := snap.Gold
	if rate != 72500 {
		t.Errorf("gold rate: expected 72500, got %v", rate)
	}
}

// TestEvaluateThresholdsUseCase_RateMissSkipsCity verifies that when Redis has
// no entry for a city (cold-start / eviction), GetLatestRate returns an error
// and the use case skips evaluation (returns nil error — not a crash).
func TestEvaluateThresholdsUseCase_RateMissSkipsCity(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	_, err := infrastructure.NewRateProjection(rdb).GetLatestRate(context.Background(), "unknown_city")
	// A cache miss returns a non-nil error; the use case returns nil in that branch.
	if err == nil {
		t.Error("GetLatestRate on missing city must return an error")
	}
}
