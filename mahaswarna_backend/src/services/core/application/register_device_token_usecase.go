package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/infrastructure"
)

type RegisterDeviceTokenUseCase struct{ repo *infrastructure.DeviceTokenRepository }

func NewRegisterDeviceTokenUseCase(r *infrastructure.DeviceTokenRepository) *RegisterDeviceTokenUseCase { return &RegisterDeviceTokenUseCase{repo: r} }

type DeviceTokenInput struct{ UserID uuid.UUID; Token, DeviceID, Platform string }

func (uc *RegisterDeviceTokenUseCase) Execute(ctx context.Context, in DeviceTokenInput) error {
	return uc.repo.Upsert(ctx, domain.DeviceToken{UserID: in.UserID, DeviceID: in.DeviceID, Token: in.Token, Platform: in.Platform})
}
