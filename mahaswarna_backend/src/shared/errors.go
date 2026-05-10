package shared

import "errors"

var (
	ErrOTPInvalid            = errors.New("otp_invalid")
	ErrDeviceNotTrusted      = errors.New("device_not_trusted")
	ErrIntegrityTokenExpired = errors.New("integrity_token_expired")
	ErrTokenExpired          = errors.New("token_expired")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidConsentType    = errors.New("invalid_consent_type")
	ErrNoActiveSubscription  = errors.New("no_active_subscription")
	ErrTooManyRequests       = errors.New("too_many_requests")
	ErrTooManyOTPRequests    = errors.New("too_many_otp_requests")
)
