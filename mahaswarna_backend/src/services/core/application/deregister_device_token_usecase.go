package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/infrastructure"
)

// DeregisterDeviceTokenUseCase removes an FCM token for a user.
// Called by DELETE /engagement/device-token/{token}.
//
// Idempotent: deleting a token that does not exist is not an error.
// The token is scoped to the authenticated userID to prevent cross-user removal.
type DeregisterDeviceTokenUseCase struct{ repo *infrastructure.DeviceTokenRepository }

func NewDeregisterDeviceTokenUseCase(r *infrastructure.DeviceTokenRepository) *DeregisterDeviceTokenUseCase {
	return &DeregisterDeviceTokenUseCase{repo: r}
}

func (uc *DeregisterDeviceTokenUseCase) Execute(ctx context.Context, userID uuid.UUID, token string) error {
	return uc.repo.Delete(ctx, userID, token)
}
