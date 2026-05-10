package http

// FeatureFlagsResponse is the canonical public flags payload.
// KillSwitch keys omit the "kill_switch_" prefix (e.g. "ws", "ai").
// CRITICAL: kill_switch_image_search defaults to true if absent from DB.
type FeatureFlagsResponse struct {
	Flags      map[string]bool    `json:"flags"`
	KillSwitch map[string]bool    `json:"killSwitch"`
	Params     map[string]float64 `json:"params"`
}
