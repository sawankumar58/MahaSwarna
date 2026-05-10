package application

import (
	"context"
	"time"

	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	ce "github.com/mahaswarna/contracts/events"
)

type SetFlagUseCase struct {
	repo     *infrastructure.FlagsRepository
	audit    *infrastructure.AuditLogRepository
	notifier *events.Notifier
}

func NewSetFlagUseCase(r *infrastructure.FlagsRepository, a *infrastructure.AuditLogRepository, n *events.Notifier) *SetFlagUseCase {
	return &SetFlagUseCase{repo: r, audit: a, notifier: n}
}

func (uc *SetFlagUseCase) SetFlag(ctx context.Context, actor, key, value string) error {
	if err := uc.repo.Set(ctx, key, value); err != nil { return err }
	uc.notifier.Notify(ctx, ce.ChannelFlagUpdated, ce.FlagUpdatedPayload{Key: key, Value: value})
	uc.audit.Append(ctx, shared.AuditEntry{Actor: actor, Action: "flag_updated", Entity: "feature_flags", EntityID: key, Metadata: map[string]any{"value": value}})
	return nil
}

// ActivateWSKillSwitch enforces OQ-8 ordering: raise BFF rate limit FIRST, sleep 5s, THEN flip kill-switch.
func (uc *SetFlagUseCase) ActivateWSKillSwitch(ctx context.Context) error {
	if err := uc.SetFlag(ctx, "system", "rate_limit_bff_free_rpm", "60"); err != nil { return err }
	time.Sleep(5 * time.Second)
	return uc.SetFlag(ctx, "system", "kill_switch_ws", "true")
}
