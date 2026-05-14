package events

// Notifier is a re-export of the canonical pgnotify.Notifier.
// Intelligence service currently only calls Notify(ctx, channel, payload) —
// the same signature as pgnotify.Notifier. This alias removes the duplicate
// pg_notify implementation while preserving the package-local type name used
// by listeners.go and any future callers.

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/infrastructure/pgnotify"
)

// Notifier delegates entirely to pgnotify.Notifier.
// SECURITY: channel must always be a compile-time constant from contracts/events — never
// accept a caller-supplied channel string.
type Notifier = pgnotify.Notifier

// NewNotifier creates a Notifier for the intelligence service.
func NewNotifier(pool *pgxpool.Pool) *Notifier {
	return pgnotify.NewNotifier(pool)
}
