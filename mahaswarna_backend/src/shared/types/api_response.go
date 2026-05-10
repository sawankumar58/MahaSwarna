package types

// APIResponse is the canonical JSON envelope for all HTTP responses.
//
// Success:
//
//	{"ok": true,  "data": <payload>}
//
// Error:
//
//	{"ok": false, "error": {"code": "otp_invalid", "message": "OTP is incorrect or expired"}}
type APIResponse[T any] struct {
	OK    bool       `json:"ok"`
	Data  *T         `json:"data,omitempty"`
	Error *APIError  `json:"error,omitempty"`
}

// APIError is the error sub-object returned when ok=false.
// Code matches one of the sentinel strings in shared/errors.go.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK constructs a successful envelope.
func Success[T any](data T) APIResponse[T] {
	return APIResponse[T]{OK: true, Data: &data}
}

// Fail constructs an error envelope.
func Fail[T any](code, message string) APIResponse[T] {
	return APIResponse[T]{OK: false, Error: &APIError{Code: code, Message: message}}
}
