package jobs

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	ce "github.com/mahaswarna/contracts/events"
)

type SubscriptionExpiryJob struct{ subRepo *infrastructure.SubscriptionRepository; notifier *events.Notifier }

func NewSubscriptionExpiryJob(s *infrastructure.SubscriptionRepository, n *events.Notifier) *SubscriptionExpiryJob {
	return &SubscriptionExpiryJob{subRepo: s, notifier: n}
}

func (j *SubscriptionExpiryJob) Register(c *cron.Cron) {
	c.AddFunc("5 0 * * *", func() {
		ctx := context.Background()
		n, err := j.subRepo.ExpireOverdue(ctx)
		if err != nil { slog.Error("sub expiry job failed", "err", err); return }
		slog.Info("subscriptions expired", "count", n)
		if n > 0 {
			j.notifier.Notify(ctx, ce.ChannelSubscriptionExpired, ce.SubscriptionExpiredPayload{UserID: "bulk", Tier: "PREMIUM"})
		}
	})
}
