package domain

import "time"
import "github.com/google/uuid"

type DeviceToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	DeviceID  string
	Token     string
	Platform  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
