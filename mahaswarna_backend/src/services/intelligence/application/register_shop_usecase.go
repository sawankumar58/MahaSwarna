package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// RegisterShopUseCase creates a shop profile for a PREMIUM user.
// PREMIUM check uses SubscriptionProjection (Redis read model from core events).
type RegisterShopUseCase struct {
	shops   *infrastructure.ShopRepository
	subProj *infrastructure.SubscriptionProjection
}

func NewRegisterShopUseCase(
	shops *infrastructure.ShopRepository,
	subProj *infrastructure.SubscriptionProjection,
) *RegisterShopUseCase {
	return &RegisterShopUseCase{shops: shops, subProj: subProj}
}

type RegisterShopInput struct {
	UserID  uuid.UUID
	Name    string
	Address string
	GST     string
	Phone   string
}

func (uc *RegisterShopUseCase) Execute(ctx context.Context, in RegisterShopInput) (*domain.Shop, error) {
	// 1. Guard: PREMIUM only.
	premium, err := uc.subProj.IsPremium(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("subscription check: %w", err)
	}
	if !premium {
		return nil, domain.ErrNotPremium{}
	}

	// 2. Validate GSTIN format.
	if !domain.ValidateGSTIN(in.GST) {
		return nil, fmt.Errorf("invalid GSTIN format: %s", in.GST)
	}

	// 3. Insert (DB unique constraint on user_id returns ErrShopAlreadyExists).
	shop, err := uc.shops.Insert(ctx, domain.Shop{
		UserID:    in.UserID,
		Name:      in.Name,
		Address:   in.Address,
		GSTNumber: in.GST,
		Phone:     in.Phone,
	})
	if err != nil {
		return nil, err // ErrShopAlreadyExists propagates as-is
	}
	return shop, nil
}
