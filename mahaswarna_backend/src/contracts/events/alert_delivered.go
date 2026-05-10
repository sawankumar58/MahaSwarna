package events

const ChannelAlertDelivered = "alert_delivered"

type AlertDeliveredPayload struct {
	AlertID string  `json:"alert_id"`
	UserID  string  `json:"user_id"`
	CityID  string  `json:"city_id"`
	Metal   string  `json:"metal"`
	Rate    float64 `json:"rate"`
}
