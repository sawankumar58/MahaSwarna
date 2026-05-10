package domain

import "time"
import "github.com/google/uuid"

type Session struct {
	JTI       uuid.UUID
	UserID    uuid.UUID
	Revoked   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}
