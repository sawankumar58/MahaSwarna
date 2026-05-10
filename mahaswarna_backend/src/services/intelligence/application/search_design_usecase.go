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

// RecommendDesignUseCase → recommend_design_usecase.go
