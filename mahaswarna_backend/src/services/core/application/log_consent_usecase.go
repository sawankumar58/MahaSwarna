package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
)

type LogConsentUseCase struct{ repo *infrastructure.ConsentLogRepository }

func NewLogConsentUseCase(r *infrastructure.ConsentLogRepository) *LogConsentUseCase { return &LogConsentUseCase{repo: r} }

type ConsentInput struct{ UserID uuid.UUID; ConsentType, Version string }

func (uc *LogConsentUseCase) Execute(ctx context.Context, in ConsentInput) (bool, error) {
	if !domain.ValidConsentTypes[in.ConsentType] {
		return false, shared.ErrInvalidConsentType
	}
	return uc.repo.Upsert(ctx, domain.ConsentLog{UserID: in.UserID, ConsentType: in.ConsentType, Version: in.Version})
}
