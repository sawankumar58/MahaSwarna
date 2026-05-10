package domain

import "time"
import "github.com/google/uuid"

type Subscription struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Tier          string
	PurchaseToken string
	ProductID     string
	PackageName   string
	Status        string
	ActivatedAt   time.Time
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const (SubscriptionStatusActive = "ACTIVE"; SubscriptionStatusExpired = "EXPIRED"; SubscriptionStatusCancelled = "CANCELLED")
