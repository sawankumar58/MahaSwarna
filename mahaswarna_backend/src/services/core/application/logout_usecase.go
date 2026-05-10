package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
)

type LogoutUseCase struct{ sessions *infrastructure.SessionRepository }

func NewLogoutUseCase(s *infrastructure.SessionRepository) *LogoutUseCase { return &LogoutUseCase{sessions: s} }

func (uc *LogoutUseCase) Execute(ctx context.Context, refreshToken string) error {
	jti, err := uuid.Parse(refreshToken)
	if err != nil { return shared.ErrUnauthorized }
	return uc.sessions.Revoke(ctx, jti)
}
