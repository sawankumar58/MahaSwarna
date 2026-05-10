package domain

import "time"
import "github.com/google/uuid"

type ConsentLog struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ConsentType string
	Version     string
	ConsentedAt time.Time
}

const (ConsentTypePrivacyPolicy = "privacy_policy"; ConsentTypeTOS = "tos")

// ValidConsentTypes allowlist enforced by log_consent_usecase.go.
// "ai_disclaimer" is intentionally absent — it is NEVER a valid ConsentType.
var ValidConsentTypes = map[string]bool{
	ConsentTypePrivacyPolicy: true,
	ConsentTypeTOS:           true,
}
