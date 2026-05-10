package domain

import "time"
import "github.com/google/uuid"

type User struct {
	ID            uuid.UUID
	Phone         string
	CityID        string
	Tier          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
	HardDeletedAt *time.Time
}

const (TierFree = "FREE"; TierPremium = "PREMIUM"; TierAdmin = "ADMIN")
