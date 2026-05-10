package events

import "time"

const ChannelAccountDeleted = "account_deleted"

type AccountDeletedPayload struct {
	UserID      string    `json:"user_id"`
	DeletedAt   time.Time `json:"deleted_at"`
	RequestedAt time.Time `json:"requested_at"`
}
