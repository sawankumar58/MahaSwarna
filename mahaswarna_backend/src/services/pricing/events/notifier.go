package events

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	contractsevents "github.com/mahaswarna/contracts/events"
	"github.com/mahaswarna/infrastructure/pgnotify"
)

// Notifier fires PostgreSQL NOTIFY on rate domain events.
// Embeds pgnotify.Notifier so the base Notify() method is not re-implemented here.
// Subscribers (redis_fanout.go, core alerts job) LISTEN on these channels.
type Notifier struct {
	*pgnotify.Notifier
}

// NewNotifier creates a pricing Notifier backed by the canonical pgnotify implementation.
func NewNotifier(db *pgxpool.Pool) *Notifier {
	return &Notifier{Notifier: pgnotify.NewNotifier(db)}
}

// NotifyRateUpdated fires after a successful rate write.
// Consumed by redis_fanout.go → WebSocket clients and core alerts threshold evaluator.
func (n *Notifier) NotifyRateUpdated(ctx context.Context, cityID string, gold, silver float64, stale bool) error {
	return n.Notify(ctx, contractsevents.ChannelRateUpdated, contractsevents.RateUpdatedPayload{
		CityID: cityID,
		Gold:   gold,
		Silver: silver,
		Stale:  stale,
	})
}

// NotifyRateStale fires when a city's snapshot is marked stale.
// Consumed by Alertmanager → SEV-2 PagerDuty integration.
func (n *Notifier) NotifyRateStale(ctx context.Context, cityID, metal, reason string) error {
	return n.Notify(ctx, contractsevents.ChannelRateStale, contractsevents.RateStalePayload{
		CityID: cityID,
		Metal:  metal,
		Reason: reason,
	})
}

// NotifyAIRateSnapshotReady fires after a new Gemini snapshot is persisted.
// The pg trigger trg_rate_snapshot_notify also fires this NOTIFY; this explicit call
// is belt-and-suspenders to ensure the fanout fires even if the trigger is missing
// (e.g. during migration or cold-start before trigger is applied).
// Consumed by: redis_fanout.go → BufferedFanout → WebSocket clients (rates channel).
func (n *Notifier) NotifyAIRateSnapshotReady(ctx context.Context, cityID string, gold, silver float64, stale bool, source string) error {
	return n.Notify(ctx, contractsevents.ChannelAIRateSnapshotReady, contractsevents.AIRateSnapshotReadyPayload{
		CityID: cityID,
		Gold:   gold,
		Silver: silver,
		Stale:  stale,
		Source: source,
	})
}
