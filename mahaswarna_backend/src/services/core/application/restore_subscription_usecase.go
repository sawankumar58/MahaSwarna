package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
)

type RestoreSubscriptionUseCase struct {
	subRepo *infrastructure.SubscriptionRepository
	play    *infrastructure.GooglePlayClient
}

func NewRestoreSubscriptionUseCase(s *infrastructure.SubscriptionRepository, p *infrastructure.GooglePlayClient) *RestoreSubscriptionUseCase {
	return &RestoreSubscriptionUseCase{subRepo: s, play: p}
}

func (uc *RestoreSubscriptionUseCase) Execute(ctx context.Context, userID uuid.UUID) (*BillingOutput, error) {
	sub, err := uc.subRepo.GetActiveByUserID(ctx, userID)
	if err != nil { return nil, err }
	if sub == nil { return nil, shared.ErrNoActiveSubscription }
	exp := ""
	if sub.ExpiresAt != nil { exp = sub.ExpiresAt.Format("2006-01-02T15:04:05Z07:00") }
	return &BillingOutput{Tier: sub.Tier, ExpiresAt: exp}, nil
}
