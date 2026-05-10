package application

import (
	"context"

	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// RecommendDesignUseCase returns trending designs ordered by view count.
//
// Lightweight heuristic — no ML inference. Delegates to DesignRepository.Search
// with an empty Query, which falls back to ORDER BY view_count DESC in the
// repository layer. A future iteration can swap in vector-similarity without
// changing this interface.
//
// View counts are maintained by ViewCountCache (Redis INCR) and flushed
// periodically by FlushViewCountsJob — they are never incremented in Postgres
// per-request, so this read path has no write-amplification concern.
type RecommendDesignUseCase struct {
	designs *infrastructure.DesignRepository
}

func NewRecommendDesignUseCase(designs *infrastructure.DesignRepository) *RecommendDesignUseCase {
	return &RecommendDesignUseCase{designs: designs}
}

// RecommendInput carries optional filters for the trending query.
type RecommendInput struct {
	Region    string           // empty → no region filter (all-India results)
	MetalType domain.MetalType // empty → no metal filter
	Limit     int              // max results; clamped to [1, 20]
}

// Recommend returns up to Limit trending designs matching the given filters.
// Limit is clamped to 20 server-side; zero or negative defaults to 20.
func (uc *RecommendDesignUseCase) Recommend(ctx context.Context, in RecommendInput) ([]domain.Design, error) {
	if in.Limit <= 0 || in.Limit > 20 {
		in.Limit = 20
	}
	result, err := uc.designs.Search(ctx, domain.SearchParams{
		Region:    in.Region,
		MetalType: in.MetalType,
		Page:      1,
		PageSize:  in.Limit,
		// Empty Query → repository falls back to ORDER BY view_count DESC.
	})
	if err != nil {
		return nil, err
	}
	return result.Designs, nil
}
