package events

const ChannelRateUpdated = "rate_updated"

type RateUpdatedPayload struct {
	CityID string  `json:"city_id"`
	Gold   float64 `json:"gold"`
	Silver float64 `json:"silver"`
	Stale  bool    `json:"stale"`
}
