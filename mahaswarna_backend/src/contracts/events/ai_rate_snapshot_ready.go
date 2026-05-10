package events

const ChannelAIRateSnapshotReady = "ai_rate_snapshot_ready"

type AIRateSnapshotReadyPayload struct {
	CityID string  `json:"city_id"`
	Gold   float64 `json:"gold"`
	Silver float64 `json:"silver"`
	Stale  bool    `json:"stale"`
	Source string  `json:"source"` // "live" | "stale" | "manual_override"
}
