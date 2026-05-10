package application

import (
	"context"
	"fmt"

	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/pricing/infrastructure"
)

// GetHistoryUseCase returns historical rate snapshots for a city.
type GetHistoryUseCase struct {
	snapRepo *infrastructure.AIRateSnapshotRepository
}

func NewGetHistoryUseCase(snapRepo *infrastructure.AIRateSnapshotRepository) *GetHistoryUseCase {
	return &GetHistoryUseCase{snapRepo: snapRepo}
}

// Execute returns up to limit snapshots for cityID, ordered newest first.
func (uc *GetHistoryUseCase) Execute(ctx context.Context, cityID string, limit int) ([]*domain.AIRateSnapshot, error) {
	snaps, err := uc.snapRepo.GetHistory(ctx, cityID, limit)
	if err != nil {
		return nil, fmt.Errorf("get history %s: %w", cityID, err)
	}
	return snaps, nil
}
