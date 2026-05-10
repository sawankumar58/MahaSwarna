package http

// BFFHomeResponse is the aggregated home-screen payload assembled by the gateway.
// _degraded is set true when any upstream call partially failed.
type BFFHomeResponse struct {
	Rate      *RateResponse      `json:"rate,omitempty"`
	AIRate    *AIRateResponse    `json:"aiRate,omitempty"`
	Flags     *FeatureFlagsResponse `json:"flags,omitempty"`
	Alerts    []AlertResponse    `json:"alerts,omitempty"`
	Degraded  bool               `json:"_degraded,omitempty"`
}
