package events

// Notifier is a re-export of the canonical pgnotify.Notifier.
// Core service application code uses *events.Notifier; this alias preserves
// backward compatibility while eliminating the duplicate pg_notify implementation.
//
// All application-layer callers (delete_account, verify_receipt, deliver_alert,
// set_flag, jobs) continue to use events.NewNotifier(db) and events.Notifier
// without any import changes.

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/infrastructure/pgnotify"
)

// Notifier wraps pgnotify.Notifier so the core events package exposes its own type.
// All Notify calls delegate to the canonical infrastructure implementation.
type Notifier = pgnotify.Notifier

// NewNotifier creates a Notifier for the core service.
// Delegates to pgnotify.NewNotifier — no duplicate pg_notify logic here.
func NewNotifier(pool *pgxpool.Pool) *Notifier {
	return pgnotify.NewNotifier(pool)
}
