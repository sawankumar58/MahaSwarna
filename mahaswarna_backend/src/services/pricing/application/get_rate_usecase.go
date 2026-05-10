package application

import (
	"context"
	"fmt"

	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/pricing/infrastructure"
)

// GetRateUseCase reads the latest rate for a city.
// Read path: Redis cache → DB fallback → 404 (cold-start / Gemini-down edge case).
type GetRateUseCase struct {
	cache    *infrastructure.RateCache
	snapRepo *infrastructure.AIRateSnapshotRepository
}

func NewGetRateUseCase(
	cache *infrastructure.RateCache,
	snapRepo *infrastructure.AIRateSnapshotRepository,
) *GetRateUseCase {
	return &GetRateUseCase{cache: cache, snapRepo: snapRepo}
}

// ErrRateNotAvailable is returned when no snapshot exists in Redis or DB.
// The HTTP handler maps this to 404 with body {"error":"city_rates_not_available"}.
// This only occurs on first-ever startup before Gemini has run its first fetch,
// or when Gemini has been unreachable since startup.
var ErrRateNotAvailable = fmt.Errorf("city_rates_not_available")

// Execute returns the latest snapshot for cityID.
// stale:true is set if: (a) the snapshot itself is marked stale, or
// (b) the DB fallback is served because Redis was cold.
func (uc *GetRateUseCase) Execute(ctx context.Context, cityID string) (*domain.AIRateSnapshot, error) {
	// 1. Redis — primary path.
	snap, err := uc.cache.Get(ctx, cityID)
	if err != nil {
		// Redis error: fall through to DB (degraded mode).
		snap = nil
	}

	if snap != nil {
		return snap, nil
	}

	// 2. DB fallback (Redis cold or Redis error).
	snap, err = uc.snapRepo.GetLatest(ctx, cityID)
	if err != nil {
		return nil, fmt.Errorf("get rate %s db: %w", cityID, err)
	}

	if snap == nil {
		// Cold-start edge case: no snapshot exists anywhere.
		// ARCHITECTURE NOTE: return 404, not 503. 503 implies retry-able transient;
		// 404 signals "not yet available" so the client shows an informational state.
		return nil, ErrRateNotAvailable
	}

	// DB fallback is always treated as stale — the Redis TTL would have kept it fresh.
	snap.IsStale = true
	return snap, nil
}
