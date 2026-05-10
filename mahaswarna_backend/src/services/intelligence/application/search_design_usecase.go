package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// SearchDesignUseCase handles paginated FTS catalog searches and single-design fetch.
type SearchDesignUseCase struct {
	designs    *infrastructure.DesignRepository
	viewCounts *infrastructure.ViewCountCache
}

func NewSearchDesignUseCase(
	designs *infrastructure.DesignRepository,
	viewCounts *infrastructure.ViewCountCache,
) *SearchDesignUseCase {
	return &SearchDesignUseCase{designs: designs, viewCounts: viewCounts}
}

// Search performs full-text search and returns a paginated result.
func (uc *SearchDesignUseCase) Search(ctx context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
	return uc.designs.Search(ctx, params)
}

// GetAndTrackView returns a single design by ID and buffers a view count increment in Redis.
func (uc *SearchDesignUseCase) GetAndTrackView(ctx context.Context, id uuid.UUID) (*domain.Design, error) {
	design, err := uc.designs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Buffer the view increment; do not block on Redis failure.
	if incErr := uc.viewCounts.Increment(ctx, id); incErr != nil {
		// Non-fatal: log and continue; view counts are eventually consistent.
		_ = fmt.Errorf("view count increment (non-fatal): %w", incErr)
	}
	return design, nil
}

// RecommendDesignUseCase returns trending designs for a region based on view counts.
// This is a lightweight heuristic: no ML inference, just DB ORDER BY view_count DESC.
// A future iteration can replace this with a vector-similarity approach.
type RecommendDesignUseCase struct {
	designs *infrastructure.DesignRepository
}

func NewRecommendDesignUseCase(designs *infrastructure.DesignRepository) *RecommendDesignUseCase {
	return &RecommendDesignUseCase{designs: designs}
}

type RecommendInput struct {
	Region    string            // filters by region (or all regions if empty)
	MetalType domain.MetalType  // "" = no metal filter
	Limit     int               // max results; capped at 20
}

// Recommend returns up to Limit trending designs for the given region.
func (uc *RecommendDesignUseCase) Recommend(ctx context.Context, in RecommendInput) ([]domain.Design, error) {
	if in.Limit <= 0 || in.Limit > 20 {
		in.Limit = 20
	}
	result, err := uc.designs.Search(ctx, domain.SearchParams{
		Region:    in.Region,
		MetalType: in.MetalType,
		Page:      1,
		PageSize:  in.Limit,
		// Empty query → fallback to view_count DESC ordering in repository.
	})
	if err != nil {
		return nil, err
	}
	return result.Designs, nil
}
