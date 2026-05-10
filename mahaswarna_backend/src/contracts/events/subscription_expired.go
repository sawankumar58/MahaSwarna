package events

const ChannelSubscriptionExpired = "subscription_expired"

type SubscriptionExpiredPayload struct {
	UserID string `json:"user_id"`
	Tier   string `json:"tier"`
}
