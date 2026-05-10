package pgnotify

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler is called for each received notification on a channel.
// channel is the PostgreSQL NOTIFY channel name; payload is the raw JSON string.
type Handler func(ctx context.Context, channel, payload string)

// Listener wraps pgx LISTEN/NOTIFY with automatic reconnect and startup catch-up.
//
// ARCHITECTURE INVARIANT (from ARCHITECTURE.md §Event Bus):
// The onReconnect callback is called on EVERY reconnect, not only at startup.
// PostgreSQL NOTIFY is fire-and-forget — events emitted during a reconnect window
// are permanently lost, so subscribers must re-run their catch-up queries each time.
type Listener struct {
	pool        *pgxpool.Pool
	channels    []string
	onReconnect func(ctx context.Context) error
	handlers    map[string][]Handler
}

// NewListener creates a Listener subscribed to the given channels.
// onReconnect is called at startup and on every reconnect — it must re-run
// the catch-up query to recover any events lost during the gap.
func NewListener(pool *pgxpool.Pool, channels []string, onReconnect func(ctx context.Context) error) *Listener {
	return &Listener{
		pool:        pool,
		channels:    channels,
		onReconnect: onReconnect,
		handlers:    make(map[string][]Handler),
	}
}

// On registers a handler for a specific channel.
func (l *Listener) On(channel string, h Handler) {
	l.handlers[channel] = append(l.handlers[channel], h)
}

// Listen blocks, processing notifications. It reconnects automatically on error.
// Call in a goroutine: go listener.Listen(ctx)
func (l *Listener) Listen(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := l.listen(ctx); err != nil {
			slog.Warn("pgnotify listener disconnected, reconnecting", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (l *Listener) listen(ctx context.Context) error {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// SECURITY NOTE: channel names are internal constants from contracts/events
	// (e.g. ChannelRateUpdated = "rate_updated"). They are never derived from
	// external input, so string concatenation here does not introduce SQL injection.
	// Do not pass user-supplied strings to NewListener.
	for _, ch := range l.channels {
		if _, err := conn.Exec(ctx, "LISTEN "+ch); err != nil {
			return err
		}
	}

	// Catch-up on reconnect — mandatory invariant.
	if l.onReconnect != nil {
		if err := l.onReconnect(ctx); err != nil {
			slog.Error("pgnotify reconnect catch-up failed", "err", err)
		}
	}

	for {
		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		handlers := l.handlers[notif.Channel]
		for _, h := range handlers {
			h(ctx, notif.Channel, notif.Payload)
		}
	}
}

// Unmarshal is a convenience helper to decode a notification payload into a typed struct.
func Unmarshal[T any](payload string) (T, error) {
	var v T
	err := json.Unmarshal([]byte(payload), &v)
	return v, err
}
