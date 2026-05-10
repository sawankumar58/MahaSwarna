package types

import "time"

// EventEnvelope wraps every pgnotify payload with routing metadata.
// Services may use the bare payload structs from contracts/events directly,
// but when a channel carries multiple event kinds the envelope lets handlers
// dispatch without deserialising the inner payload twice.
//
// PostgreSQL NOTIFY payload (JSON):
//
//	{
//	  "event_id":   "a3f8...",          // random hex JTI (shared.GenerateJTI)
//	  "event_type": "rate_updated",     // matches the ChannelXxx constant
//	  "occurred_at": "2026-05-10T…",
//	  "payload":    { … }              // channel-specific struct
//	}
type EventEnvelope struct {
	EventID     string         `json:"event_id"`
	EventType   string         `json:"event_type"`
	OccurredAt  time.Time      `json:"occurred_at"`
	Payload     map[string]any `json:"payload"`
}
