package events

const ChannelRateStale = "rate_stale"

type RateStalePayload struct {
	CityID string `json:"city_id"`
	Metal  string `json:"metal"`
	Reason string `json:"reason"` // "timeout" | "sanity_fail" | "consecutive_fail"
}
