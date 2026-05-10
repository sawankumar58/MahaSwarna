package application

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/pricing/events"
	"github.com/mahaswarna/pricing/infrastructure"
	"golang.org/x/sync/errgroup"
)

const (
	// geminiConcurrency limits simultaneous Gemini API calls per scheduler run.
	// Avoids rate-limit errors while still completing all 61 cities in reasonable time.
	geminiConcurrency = 5

	// geminiCityTimeout is the per-city deadline for a Gemini API call.
	geminiCityTimeout = 2 * time.Second

	// defaultSanityThresholdPct is the maximum allowed rate change between runs (2%).
	// Configurable via feature flag "rate_sanity_threshold_pct".
	// 2% is tighter than 5% because gold intraday moves are typically 0.3–1.0%.
	// A 5% gate would silently accept ₹300/gram errors on a ₹6000/gram price.
	defaultSanityThresholdPct = 0.02

	// consecutiveFailLimit triggers SEV-1 escalation and WS kill-switch.
	consecutiveFailLimit = 3
)

// FlagReader allows the use case to read the rate_sanity_threshold_pct feature flag
// without importing the full flags infrastructure.
type FlagReader interface {
	GetFloat(ctx context.Context, key string, defaultVal float64) float64
}

// FlagWriter allows the use case to update feature flags (for the 3-failure kill-switch path).
type FlagWriter interface {
	SetFlag(ctx context.Context, key, value string) error
}

// GenerateAIRatesUseCase fetches gold and silver rates from Gemini for all active cities.
// It is called by ai_rate_scheduler_job.go on every IST market-hours tick.
type GenerateAIRatesUseCase struct {
	gemini           *infrastructure.GeminiClient
	snapRepo         *infrastructure.AIRateSnapshotRepository
	cache            *infrastructure.RateCache
	notifier         *events.Notifier
	flagReader       FlagReader
	flagWriter       FlagWriter
	consecutiveFails int
	mu               sync.Mutex // guards consecutiveFails
}

func NewGenerateAIRatesUseCase(
	gemini *infrastructure.GeminiClient,
	snapRepo *infrastructure.AIRateSnapshotRepository,
	cache *infrastructure.RateCache,
	notifier *events.Notifier,
	flagReader FlagReader,
	flagWriter FlagWriter,
) *GenerateAIRatesUseCase {
	return &GenerateAIRatesUseCase{
		gemini:     gemini,
		snapRepo:   snapRepo,
		cache:      cache,
		notifier:   notifier,
		flagReader: flagReader,
		flagWriter: flagWriter,
	}
}

// Execute fetches rates for all active cities with bounded concurrency.
// On partial failure: affected cities retain their previous snapshot with stale:true.
// On 3 consecutive full-run failures: triggers SEV-1 escalation and WS kill-switch.
func (uc *GenerateAIRatesUseCase) Execute(ctx context.Context) error {
	cities, err := uc.snapRepo.GetActiveCities(ctx)
	if err != nil {
		return fmt.Errorf("get active cities: %w", err)
	}

	sem := make(chan struct{}, geminiConcurrency)
	g, gCtx := errgroup.WithContext(ctx)
	failCount := 0
	var failMu sync.Mutex

	for _, city := range cities {
		city := city // loop var capture
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := uc.generateForCity(gCtx, city.ID); err != nil {
				slog.Warn("gemini city failed", "city", city.ID, "err", err)
				failMu.Lock()
				failCount++
				failMu.Unlock()
				// Per-city failures are non-fatal for the run — other cities proceed.
				return nil
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	if failCount == len(cities) {
		// Full run failure — all cities failed.
		uc.mu.Lock()
		uc.consecutiveFails++
		n := uc.consecutiveFails
		uc.mu.Unlock()

		slog.Error("gemini full run failure", "consecutive", n, "cities", len(cities))
		sentry.CaptureMessage(fmt.Sprintf("pricing: Gemini full-run failure (%d consecutive)", n))

		if n >= consecutiveFailLimit {
			uc.triggerKillSwitch(ctx)
		}
	} else {
		// At least one city succeeded — reset the consecutive failure counter.
		uc.mu.Lock()
		uc.consecutiveFails = 0
		uc.mu.Unlock()
	}

	return nil
}

// generateForCity fetches a rate for one city, runs sanity checks, then persists.
func (uc *GenerateAIRatesUseCase) generateForCity(ctx context.Context, cityID string) error {
	cityCtx, cancel := context.WithTimeout(ctx, geminiCityTimeout)
	defer cancel()

	result, err := uc.gemini.GenerateRate(cityCtx, cityID)
	if err != nil {
		// Gemini failure: preserve last snapshot with stale flag.
		_ = uc.markCityStale(ctx, cityID, "gold,silver", "timeout")
		return fmt.Errorf("gemini city %s: %w", cityID, err)
	}

	// Sanity check: reject anomalous rate changes before persisting.
	threshold := uc.flagReader.GetFloat(ctx, "rate_sanity_threshold_pct", defaultSanityThresholdPct)
	staleMetal, sanityErr := uc.sanityCheck(ctx, cityID, result.Gold, result.Silver, threshold)
	if sanityErr != nil {
		_ = uc.markCityStale(ctx, cityID, staleMetal, "sanity_fail")
		slog.Warn("sanity check failed", "city", cityID, "err", sanityErr)
		sentry.CaptureException(fmt.Errorf("pricing sanity check %s: %w", cityID, sanityErr))
		return sanityErr
	}

	// Persist snapshot — pg trigger fires NOTIFY ai_rate_snapshot_ready.
	snap := &domain.AIRateSnapshot{
		CityID:      cityID,
		Gold:        result.Gold,
		Silver:      result.Silver,
		Source:      domain.SourceGemini,
		IsStale:     false,
		GeneratedAt: time.Now().UTC(),
	}

	if err := uc.snapRepo.InsertSnapshot(ctx, snap); err != nil {
		return fmt.Errorf("insert snapshot %s: %w", cityID, err)
	}

	// Warm Redis immediately so the next GET hits cache, not DB.
	if err := uc.cache.Set(ctx, snap); err != nil {
		slog.Warn("redis cache set failed", "city", cityID, "err", err)
		// Non-fatal: DB fallback will serve the rate.
	}

	// Belt-and-suspenders: explicitly fire ai_rate_snapshot_ready so the
	// BufferedFanout pushes to WS clients even if the pg trigger is absent.
	if err := uc.notifier.NotifyAIRateSnapshotReady(ctx, cityID, snap.Gold, snap.Silver, false, string(domain.SourceGemini)); err != nil {
		slog.Warn("notify ai_rate_snapshot_ready failed", "city", cityID, "err", err)
		// Non-fatal: pg trigger may still fire.
	}

	return nil
}

// sanityCheck rejects a new rate if it deviates from the previous snapshot by more
// than threshold. Returns the triggering metal name ("gold", "silver") so callers
// can pass it to NotifyRateStale with the correct label. Returns ("", nil) on pass.
func (uc *GenerateAIRatesUseCase) sanityCheck(ctx context.Context, cityID string, newGold, newSilver, threshold float64) (metal string, err error) {
	prev, cErr := uc.cache.Get(ctx, cityID)
	if cErr != nil || prev == nil {
		// No previous snapshot (cold start) — pass.
		return "", nil
	}

	goldDelta := math.Abs(newGold-prev.Gold) / prev.Gold
	silverDelta := math.Abs(newSilver-prev.Silver) / prev.Silver

	if goldDelta > threshold {
		return "gold", fmt.Errorf("gold anomaly city=%s prev=%.2f new=%.2f delta=%.4f (threshold=%.4f)",
			cityID, prev.Gold, newGold, goldDelta, threshold)
	}
	if silverDelta > threshold {
		return "silver", fmt.Errorf("silver anomaly city=%s prev=%.2f new=%.2f delta=%.4f (threshold=%.4f)",
			cityID, prev.Silver, newSilver, silverDelta, threshold)
	}
	return "", nil
}

// markCityStale sets is_stale on the latest snapshot, evicts Redis, and fires pg NOTIFY rate_stale.
// metal identifies which metal triggered the stale condition ("gold", "silver", or "gold,silver").
func (uc *GenerateAIRatesUseCase) markCityStale(ctx context.Context, cityID, metal, reason string) error {
	if err := uc.snapRepo.MarkStale(ctx, cityID); err != nil {
		slog.Warn("mark stale db failed", "city", cityID, "err", err)
	}
	if err := uc.cache.Delete(ctx, cityID); err != nil {
		slog.Warn("evict stale cache failed", "city", cityID, "err", err)
	}
	return uc.notifier.NotifyRateStale(ctx, cityID, metal, reason)
}

// triggerKillSwitch implements the OQ-8 gate from ARCHITECTURE.md:
// STEP 1: raise BFF rate limit BEFORE flipping the WS kill-switch.
// STEP 2: wait 5 seconds for Redis flag cache to refresh.
// STEP 3: flip kill_switch_ws.
// This order prevents a 429-storm when clients fall back to polling.
func (uc *GenerateAIRatesUseCase) triggerKillSwitch(ctx context.Context) {
	slog.Error("pricing: activating WS kill-switch (OQ-8 gate)", "consecutive_fails", uc.consecutiveFails)
	sentry.CaptureMessage("pricing: WS kill-switch activated after 3 consecutive Gemini failures")

	bffLimit := strconv.Itoa(60)
	if err := uc.flagWriter.SetFlag(ctx, "rate_limit_bff_free_rpm", bffLimit); err != nil {
		slog.Error("OQ-8 step1 failed: could not raise BFF rate limit", "err", err)
		// Abort — do not flip kill-switch if we could not raise the rate limit first.
		// A blind flip would cause the 429-storm OQ-8 is designed to prevent.
		return
	}

	// Step 2: wait for Redis flag cache TTL to refresh (5s).
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return
	}

	// Step 3: flip kill-switch.
	if err := uc.flagWriter.SetFlag(ctx, "kill_switch_ws", "true"); err != nil {
		slog.Error("OQ-8 step3 failed: could not set kill_switch_ws", "err", err)
		return
	}

	// Alert PagerDuty via env-configured webhook (logged here; Alertmanager picks up
	// the rate_stale pg NOTIFY for the actual PagerDuty integration).
	if key := os.Getenv("PAGERDUTY_KEY"); key != "" {
		slog.Error("SEV-1: Gemini sole rate source unavailable; WS kill-switch active",
			"pagerduty_key_set", true)
	}
}
