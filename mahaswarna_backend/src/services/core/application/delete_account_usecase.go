package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	ce "github.com/mahaswarna/contracts/events"
)

type DeleteAccountUseCase struct {
	users    *infrastructure.UserRepository
	sessions *infrastructure.SessionRepository
	audit    *infrastructure.AuditLogRepository
	notifier *events.Notifier
}

func NewDeleteAccountUseCase(u *infrastructure.UserRepository, s *infrastructure.SessionRepository,
	a *infrastructure.AuditLogRepository, n *events.Notifier) *DeleteAccountUseCase {
	return &DeleteAccountUseCase{users: u, sessions: s, audit: a, notifier: n}
}

func (uc *DeleteAccountUseCase) Execute(ctx context.Context, userID uuid.UUID) error {
	if err := uc.users.SoftDelete(ctx, userID); err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	uc.sessions.RevokeAllForUser(ctx, userID)
	now := time.Now()
	if err := uc.notifier.Notify(ctx, ce.ChannelAccountDeleted, ce.AccountDeletedPayload{
		UserID: userID.String(), DeletedAt: now, RequestedAt: now,
	}); err != nil {
		shared.Logger.Error("account_deleted notify failed", "err", err, "userID", userID)
	}
	uc.audit.Append(ctx, shared.AuditEntry{
		Actor: userID.String(), Action: "account_deleted", Entity: "users", EntityID: userID.String(),
		Metadata: map[string]any{"deleted_at": now, "initiated_by": "user"},
	})
	return nil
}
