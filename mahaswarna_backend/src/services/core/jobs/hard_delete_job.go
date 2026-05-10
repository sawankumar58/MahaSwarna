package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	ce "github.com/mahaswarna/contracts/events"
)

// HardDeleteJob purges users past the 30-day grace period.
// IDEMPOTENCY: hard_deleted_at IS NULL guard prevents re-processing.
// ORDERING: hard_deleted_at is set BEFORE pg NOTIFY account_deleted (see arch doc).
type HardDeleteJob struct {
	userRepo *infrastructure.UserRepository
	audit    *infrastructure.AuditLogRepository
	notifier *events.Notifier
}

func NewHardDeleteJob(u *infrastructure.UserRepository, a *infrastructure.AuditLogRepository, n *events.Notifier) *HardDeleteJob {
	return &HardDeleteJob{userRepo: u, audit: a, notifier: n}
}

func (j *HardDeleteJob) Register(c *cron.Cron) {
	c.AddFunc("0 1 * * *", func() {
		if err := j.run(context.Background()); err != nil {
			slog.Error("hard delete job failed", "err", err)
		}
	})
}

func (j *HardDeleteJob) run(ctx context.Context) error {
	users, err := j.userRepo.PendingHardDeletes(ctx)
	if err != nil { return err }
	for _, u := range users {
		now := time.Now()
		// Mark BEFORE notify (idempotency guard).
		if err := j.userRepo.MarkHardDeleted(ctx, u.ID); err != nil {
			slog.Error("mark hard deleted failed", "userID", u.ID, "err", err)
			continue
		}
		j.notifier.Notify(ctx, ce.ChannelAccountDeleted, ce.AccountDeletedPayload{
			UserID: u.ID.String(), DeletedAt: *u.DeletedAt, RequestedAt: now,
		})
		j.audit.Append(ctx, shared.AuditEntry{
			Actor: "system", Action: "hard_delete", Entity: "users", EntityID: u.ID.String(),
			Metadata: map[string]any{"deleted_at": u.DeletedAt, "hard_deleted_at": now},
		})
		slog.Info("user hard deleted", "userID", u.ID)
	}
	return nil
}
