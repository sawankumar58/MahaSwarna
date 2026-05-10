package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Notifier struct{ pool *pgxpool.Pool }

func NewNotifier(pool *pgxpool.Pool) *Notifier { return &Notifier{pool: pool} }

func (n *Notifier) Notify(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil { return fmt.Errorf("marshal: %w", err) }
	_, err = n.pool.Exec(ctx, "SELECT pg_notify($1,$2)", channel, string(data))
	return err
}
