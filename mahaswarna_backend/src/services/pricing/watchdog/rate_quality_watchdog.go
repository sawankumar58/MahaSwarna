package watchdog

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/mahaswarna/pricing/events"
	"github.com/mahaswarna/pricing/infrastructure"
)

const (
	// watchdogInterval is how often the watchdog scans all cities.
	watchdogInterval = 5 * time.Minute

	// istMarketOpen / istMarketClose define the trading window (Mon–Sat 10:00–19:00 IST).
	istMarketOpen  = 10
	istMarketClose = 19

	defaultSanityThreshold = 0.02 // 2% — same as generate_ai_rates_usecase.go
)

// FlagReader reads the rate_sanity_threshold_pct feature flag.
type FlagReader interface {
	GetFloat(ctx context.Context, key string, defaultVal float64) float64
}

// RateQualityWatchdog runs a single goroutine that periodically checks all city snapshots
// for staleness and sanity. It fires pg NOTIFY rate_stale on failures.
type RateQualityWatchdog struct {
	snapRepo   *infrastructure.AIRateSnapshotRepository
	cache      *infrastructure.RateCache
	notifier   *events.Notifier
	flagReader FlagReader
	istLoc     *time.Location
}

func NewRateQualityWatchdog(
	snapRepo *infrastructure.AIRateSnapshotRepository,
	cache *infrastructure.RateCache,
	notifier *events.Notifier,
	flagReader FlagReader,
) *RateQualityWatchdog {
	istLoc, _ := time.LoadLocation("Asia/Kolkata")
	return &RateQualityWatchdog{
		snapRepo:   snapRepo,
		cache:      cache,
		notifier:   notifier,
		flagReader: flagReader,
		istLoc:     istLoc,
	}
}

// Run starts the watchdog loop. Call in a goroutine: go watchdog.Run(ctx).
func (w *RateQualityWatchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(watchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan(ctx)
		}
	}
}

// scan checks all active cities for staleness and sanity.
func (w *RateQualityWatchdog) scan(ctx context.Context) {
	cities, err := w.snapRepo.GetActiveCities(ctx)
	if err != nil {
		slog.Warn("watchdog: get active cities failed", "err", err)
		return
	}

	threshold := w.flagReader.GetFloat(ctx, "rate_sanity_threshold_pct", defaultSanityThreshold)
	now := time.Now().In(w.istLoc)
	inTradingWindow := isTradingWindow(now)

	for _, city := range cities {
		w.checkCity(ctx, city.ID, inTradingWindow, threshold)
	}
}

// checkCity runs staleness and sanity checks for a single city.
func (w *RateQualityWatchdog) checkCity(ctx context.Context, cityID string, inTradingWindow bool, threshold float64) {
	snap, err := w.snapRepo.GetLatest(ctx, cityID)
	if err != nil {
		slog.Warn("watchdog: get latest failed", "city", cityID, "err", err)
		return
	}
	if snap == nil {
		// No snapshot yet — cold start, skip.
		return
	}

	// 1. STALENESS CHECK: if we're inside the trading window and the snapshot
	//    was generated more than 90 minutes ago, flag it as stale.
	if inTradingWindow {
		age := time.Since(snap.GeneratedAt)
		if age > 90*time.Minute && !snap.IsStale {
			slog.Warn("watchdog: stale snapshot detected", "city", cityID, "age", age)
			if err := w.snapRepo.MarkStale(ctx, cityID); err != nil {
				slog.Warn("watchdog: mark stale failed", "city", cityID, "err", err)
			}
			if err := w.cache.Delete(ctx, cityID); err != nil {
				slog.Warn("watchdog: evict cache failed", "city", cityID, "err", err)
			}
			// Both metals are always stale together — a snapshot covers gold+silver.
			if err := w.notifier.NotifyRateStale(ctx, cityID, "gold,silver", "timeout"); err != nil {
				slog.Warn("watchdog: notify stale failed", "city", cityID, "err", err)
			}
			return
		}
	}

	// 2. SANITY CHECK: compare against the previous snapshot in the cache.
	prev, err := w.cache.Get(ctx, cityID)
	if err != nil || prev == nil {
		return // no prior data to compare — pass
	}

	// Only check if the snap is newer than the cached one (avoid re-flagging same snap).
	if !snap.GeneratedAt.After(prev.GeneratedAt) {
		return
	}

	goldDelta := math.Abs(snap.Gold-prev.Gold) / prev.Gold
	silverDelta := math.Abs(snap.Silver-prev.Silver) / prev.Silver

	if goldDelta > threshold || silverDelta > threshold {
		// Identify which metal(s) breached the threshold for accurate Alertmanager routing.
		staleMetal := "gold,silver"
		if goldDelta <= threshold {
			staleMetal = "silver"
		} else if silverDelta <= threshold {
			staleMetal = "gold"
		}

		slog.Warn("watchdog: sanity anomaly", "city", cityID,
			"gold_delta", goldDelta, "silver_delta", silverDelta, "threshold", threshold)
		sentry.CaptureMessage("pricing watchdog: rate anomaly for " + cityID)

		if err := w.snapRepo.MarkStale(ctx, cityID); err != nil {
			slog.Warn("watchdog: mark stale failed", "city", cityID, "err", err)
		}
		if err := w.cache.Delete(ctx, cityID); err != nil {
			slog.Warn("watchdog: evict cache failed", "city", cityID, "err", err)
		}
		if err := w.notifier.NotifyRateStale(ctx, cityID, staleMetal, "sanity_fail"); err != nil {
			slog.Warn("watchdog: notify failed", "city", cityID, "err", err)
		}
	}
}

// isTradingWindow returns true if the given IST time is within Mon–Sat 10:00–19:00.
func isTradingWindow(now time.Time) bool {
	weekday := now.Weekday()
	if weekday == time.Sunday {
		return false
	}
	hour := now.Hour()
	return hour >= istMarketOpen && hour < istMarketClose
}
