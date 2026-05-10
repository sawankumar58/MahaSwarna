package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
)

type RefreshUseCase struct {
	users    *infrastructure.UserRepository
	sessions *infrastructure.SessionRepository
	login    *LoginUseCase
}

func NewRefreshUseCase(u *infrastructure.UserRepository, s *infrastructure.SessionRepository, l *LoginUseCase) *RefreshUseCase {
	return &RefreshUseCase{users: u, sessions: s, login: l}
}

func (uc *RefreshUseCase) Execute(ctx context.Context, token string) (*AuthOutput, error) {
	jti, err := uuid.Parse(token)
	if err != nil { return nil, shared.ErrUnauthorized }
	session, err := uc.sessions.GetByJTI(ctx, jti)
	if err != nil || session == nil || session.Revoked || session.ExpiresAt.Before(time.Now()) {
		return nil, shared.ErrTokenExpired
	}
	if err := uc.sessions.Revoke(ctx, jti); err != nil {
		return nil, fmt.Errorf("revoke old session: %w", err)
	}
	user, err := uc.users.GetByID(ctx, session.UserID)
	if err != nil { return nil, fmt.Errorf("get user: %w", err) }
	return uc.login.IssueTokenPair(ctx, user)
}
