package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Notifier sends pg NOTIFY events from the intelligence service.
// Currently used to emit shop_registered after a successful shop INSERT
// (the DB trigger also fires, but the application notifier ensures the
// payload includes cityID which the trigger cannot supply without a join).
type Notifier struct {
	pool *pgxpool.Pool
}

func NewNotifier(pool *pgxpool.Pool) *Notifier {
	return &Notifier{pool: pool}
}

// Notify sends a pg NOTIFY on the given channel with a JSON-encoded payload.
// SECURITY: channel must be a compile-time constant from contracts/events — never
// accept a caller-supplied channel string.
func (n *Notifier) Notify(ctx context.Context, channel string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify marshal: %w", err)
	}
	// pg_notify is safe with a parameterised payload; channel is a constant.
	_, err = n.pool.Exec(ctx,
		`SELECT pg_notify($1, $2)`, channel, string(b),
	)
	return err
}
