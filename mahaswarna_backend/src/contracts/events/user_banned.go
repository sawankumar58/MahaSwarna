package events

const ChannelUserBanned = "user_banned"

type UserBannedPayload struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}
