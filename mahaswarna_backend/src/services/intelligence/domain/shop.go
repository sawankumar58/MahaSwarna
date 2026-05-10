package domain

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// gstinRe validates Indian GSTIN: 15-character format.
// Format: 2-digit state + 10-char PAN + 1 entity + Z + 1 checksum.
var gstinRe = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z]{1}[1-9A-Z]{1}Z[0-9A-Z]{1}$`)

// Shop represents a jeweller's registered shop profile.
// Only PREMIUM-tier users may register a shop (enforced by SubscriptionProjection in infrastructure).
type Shop struct {
	ID              uuid.UUID  `db:"id"`
	UserID          uuid.UUID  `db:"user_id"`
	Name            string     `db:"name"`
	Address         string     `db:"address"`
	GSTNumber       string     `db:"gst_number"`
	Phone           string     `db:"phone"`
	BannerURL       *string    `db:"banner_url"`        // nil until confirmed via S3 moderation
	BannerObjectKey *string    `db:"banner_object_key"` // S3 key for replacement cleanup
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// ValidateGSTIN checks format only; it does not perform a live GSTIN registry lookup.
func ValidateGSTIN(gstin string) bool {
	return gstinRe.MatchString(gstin)
}

// ErrShopAlreadyExists is returned when the user already has a registered shop.
type ErrShopAlreadyExists struct{}

func (ErrShopAlreadyExists) Error() string { return "shop already registered for this user" }

// ErrNotPremium is returned when a FREE-tier user attempts a PREMIUM-only operation.
type ErrNotPremium struct{}

func (ErrNotPremium) Error() string { return "operation requires PREMIUM subscription" }

// ErrDailyLimitExceeded is returned when a shop exceeds its daily invoice quota.
// Handlers match this via errors.Is to return HTTP 429.
type ErrDailyLimitExceeded struct {
	Limit int
}

func (e ErrDailyLimitExceeded) Error() string {
	return fmt.Sprintf("daily invoice limit (%d) exceeded for today", e.Limit)
}
