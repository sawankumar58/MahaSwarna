package events

const ChannelSubscriptionActivated = "subscription_activated"

// ChannelUserCreated — core self-listens to provision a FREE subscription.
const ChannelUserCreated = "user_created"

type SubscriptionChangedPayload struct {
	UserID string `json:"user_id"`
	Tier   string `json:"tier"`
	Status string `json:"status"` // "ACTIVE" | "EXPIRED" | "CANCELLED"
}

type UserCreatedPayload struct {
	UserID string `json:"user_id"`
	CityID string `json:"city_id"`
}
