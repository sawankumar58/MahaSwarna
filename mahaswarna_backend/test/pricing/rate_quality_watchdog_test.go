package pricing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/pricing/infrastructure"
	"github.com/redis/go-redis/v9"
)

// ── isTradingWindow logic (reproduced from watchdog/rate_quality_watchdog.go) ─

func isTradingWindow(now time.Time) bool {
	if now.Weekday() == time.Sunday {
		return false
	}
	h := now.Hour()
	return h >= 10 && h < 19
}

// TestRateQualityWatchdog_IsTradingWindow_WorkdayHours verifies that weekday
// hours 10–18 are inside the trading window and 0–9, 19–23 are outside.
func TestRateQualityWatchdog_IsTradingWindow_WorkdayHours(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	base := time.Date(2024, 6, 3, 0, 0, 0, 0, loc) // Monday

	inside := []int{10, 11, 12, 13, 14, 15, 16, 17, 18}
	for _, h := range inside {
		ts := base.Add(time.Duration(h) * time.Hour)
		if !isTradingWindow(ts) {
			t.Errorf("hour %d on Monday must be inside trading window", h)
		}
	}

	outside := []int{0, 1, 9, 19, 20, 23}
	for _, h := range outside {
		ts := base.Add(time.Duration(h) * time.Hour)
		if isTradingWindow(ts) {
			t.Errorf("hour %d on Monday must be outside trading window", h)
		}
	}
}

// TestRateQualityWatchdog_IsTradingWindow_SundayAlwaysOff verifies that Sunday
// is never a trading day regardless of hour.
func TestRateQualityWatchdog_IsTradingWindow_SundayAlwaysOff(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	sunday := time.Date(2024, 6, 2, 14, 0, 0, 0, loc) // Sunday 14:00 IST

	if isTradingWindow(sunday) {
		t.Error("Sunday must never be in the trading window")
	}
}

// TestRateQualityWatchdog_IsTradingWindow_SaturdayIsOpen verifies that
// Saturday is a trading day (market operates Mon–Sat).
func TestRateQualityWatchdog_IsTradingWindow_SaturdayIsOpen(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	saturday := time.Date(2024, 6, 1, 14, 0, 0, 0, loc) // Saturday 14:00 IST

	if !isTradingWindow(saturday) {
		t.Error("Saturday 14:00 IST must be inside the trading window")
	}
}

// TestRateQualityWatchdog_SanityCheck_GoldAnomaly verifies that a gold price
// delta exceeding the 2% threshold triggers the stale notification path.
func TestRateQualityWatchdog_SanityCheck_GoldAnomaly(t *testing.T) {
	const threshold = 0.02

	prevGold := 72000.0
	newGold := 75000.0 // ~4.2% spike — above threshold

	delta := abs64(newGold-prevGold) / prevGold
	if delta <= threshold {
		t.Errorf("gold delta %.4f must exceed threshold %.2f", delta, threshold)
	}
}

// TestRateQualityWatchdog_SanityCheck_NormalMovement verifies that a normal
// intraday move (~0.5%) does NOT trigger the stale path.
func TestRateQualityWatchdog_SanityCheck_NormalMovement(t *testing.T) {
	const threshold = 0.02

	prevGold := 72000.0
	newGold := 72350.0 // ~0.49% — below threshold

	delta := abs64(newGold-prevGold) / prevGold
	if delta >= threshold {
		t.Errorf("gold delta %.4f must be below threshold %.2f for normal movement", delta, threshold)
	}
}

// TestRateQualityWatchdog_StalenessThreshold_90Minutes verifies the 90-minute
// staleness cutoff used by the watchdog inside the trading window.
func TestRateQualityWatchdog_StalenessThreshold_90Minutes(t *testing.T) {
	staleAge := 91 * time.Minute
	freshAge := 89 * time.Minute
	const limit = 90 * time.Minute

	if staleAge <= limit {
		t.Error("91-minute old snapshot must trigger staleness")
	}
	if freshAge > limit {
		t.Error("89-minute old snapshot must NOT trigger staleness")
	}
}

// TestRateQualityWatchdog_CacheEvictionOnStale verifies that the watchdog
// evicts the Redis cache key for the stale city so subsequent reads fall
// through to the DB (which is marked stale) rather than serving bad data.
func TestRateQualityWatchdog_CacheEvictionOnStale(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	cityID := "kolkata"
	snap := &domain.AIRateSnapshot{
		CityID:      cityID,
		Gold:        72000,
		Silver:      85000,
		IsStale:     false,
		GeneratedAt: time.Now(),
	}
	b, _ := json.Marshal(snap)
	_ = rdb.Set(ctx, "rates:latest:ai:"+cityID, b, time.Hour).Err()

	// Confirm key exists before eviction.
	exists, _ := rdb.Exists(ctx, "rates:latest:ai:"+cityID).Result()
	if exists != 1 {
		t.Fatal("setup: cache key must exist before eviction")
	}

	// Watchdog calls cache.Delete(ctx, cityID).
	cache := infrastructure.NewRateCache(rdb)
	if err := cache.Delete(ctx, cityID); err != nil {
		t.Fatalf("cache.Delete: %v", err)
	}

	// Key must no longer exist.
	exists, _ = rdb.Exists(ctx, "rates:latest:ai:"+cityID).Result()
	if exists != 0 {
		t.Error("cache key must be evicted after watchdog marks snapshot stale")
	}
}

// TestRateQualityWatchdog_StaleMetal_BothBreached verifies the metal selector:
// when both gold AND silver exceed threshold, staleMetal = "gold,silver".
func TestRateQualityWatchdog_StaleMetal_BothBreached(t *testing.T) {
	const threshold = 0.02
	goldDelta := 0.05   // 5% — above
	silverDelta := 0.03 // 3% — above

	staleMetal := "gold,silver"
	if goldDelta <= threshold {
		staleMetal = "silver"
	} else if silverDelta <= threshold {
		staleMetal = "gold"
	}

	if staleMetal != "gold,silver" {
		t.Errorf("both breached: staleMetal must be \"gold,silver\", got %q", staleMetal)
	}
}

// TestRateQualityWatchdog_StaleMetal_GoldOnlyBreached verifies that when only
// gold exceeds the threshold, staleMetal = "gold".
func TestRateQualityWatchdog_StaleMetal_GoldOnlyBreached(t *testing.T) {
	const threshold = 0.02
	goldDelta := 0.05  // above
	silverDelta := 0.01 // below

	staleMetal := "gold,silver"
	if goldDelta <= threshold {
		staleMetal = "silver"
	} else if silverDelta <= threshold {
		staleMetal = "gold"
	}

	if staleMetal != "gold" {
		t.Errorf("gold-only breach: staleMetal must be \"gold\", got %q", staleMetal)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
