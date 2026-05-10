package http

import (
	"encoding/json"
	"net/http"

	"github.com/mahaswarna/shared"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": errCode, "message": msg})
}

func mapError(w http.ResponseWriter, err error) {
	switch err {
	case shared.ErrOTPInvalid:            writeError(w, 401, "otp_invalid", "")
	case shared.ErrDeviceNotTrusted:      writeError(w, 403, "device_not_trusted", "")
	case shared.ErrIntegrityTokenExpired: writeError(w, 403, "integrity_token_expired", "")
	case shared.ErrTokenExpired:          writeError(w, 401, "token_expired", "")
	case shared.ErrInvalidConsentType:    writeError(w, 400, "invalid_consent_type", "")
	case shared.ErrNoActiveSubscription:  writeError(w, 404, "no_active_subscription", "")
	case shared.ErrTooManyRequests:
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "too_many_requests", "")
	case shared.ErrTooManyOTPRequests:
		w.Header().Set("Retry-After", "3600")
		writeError(w, 429, "too_many_otp_requests", "")
	default:
		writeError(w, 500, "internal_error", err.Error())
	}
}
