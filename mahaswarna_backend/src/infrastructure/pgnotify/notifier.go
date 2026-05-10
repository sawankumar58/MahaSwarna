package pgnotify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Notifier sends PostgreSQL NOTIFY messages.
// It is a thin wrapper so services can share the same Notify call pattern.
type Notifier struct{ pool *pgxpool.Pool }

// NewNotifier wraps a pool for sending NOTIFY messages.
func NewNotifier(pool *pgxpool.Pool) *Notifier { return &Notifier{pool: pool} }

// Notify marshals payload to JSON and sends SELECT pg_notify(channel, payload).
func (n *Notifier) Notify(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pgnotify marshal: %w", err)
	}
	_, err = n.pool.Exec(ctx, "SELECT pg_notify($1,$2)", channel, string(data))
	return err
}
