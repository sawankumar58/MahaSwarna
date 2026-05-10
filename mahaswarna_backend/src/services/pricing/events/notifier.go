package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	contractsevents "github.com/mahaswarna/contracts/events"
)

// Notifier fires PostgreSQL NOTIFY on rate domain events.
// Subscribers (redis_fanout.go, core alerts job) LISTEN on these channels.
type Notifier struct {
	db *pgxpool.Pool
}

func NewNotifier(db *pgxpool.Pool) *Notifier {
	return &Notifier{db: db}
}

func (n *Notifier) notify(ctx context.Context, channel string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifier marshal %s: %w", channel, err)
	}
	// SECURITY: channel is an internal constant from contracts/events — never user-supplied.
	_, err = n.db.Exec(ctx, "SELECT pg_notify($1, $2)", channel, string(b))
	return err
}

// NotifyRateUpdated fires after a successful rate write.
// Consumed by redis_fanout.go → WebSocket clients and core alerts threshold evaluator.
func (n *Notifier) NotifyRateUpdated(ctx context.Context, cityID string, gold, silver float64, stale bool) error {
	return n.notify(ctx, contractsevents.ChannelRateUpdated, contractsevents.RateUpdatedPayload{
		CityID: cityID,
		Gold:   gold,
		Silver: silver,
		Stale:  stale,
	})
}

// NotifyRateStale fires when a city's snapshot is marked stale.
// Consumed by Alertmanager → SEV-2 PagerDuty integration.
func (n *Notifier) NotifyRateStale(ctx context.Context, cityID, metal, reason string) error {
	return n.notify(ctx, contractsevents.ChannelRateStale, contractsevents.RateStalePayload{
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
	return n.notify(ctx, contractsevents.ChannelAIRateSnapshotReady, contractsevents.AIRateSnapshotReadyPayload{
		CityID: cityID,
		Gold:   gold,
		Silver: silver,
		Stale:  stale,
		Source: source,
	})
}
