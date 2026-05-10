package http

type ConsentLogRequest struct {
	ConsentType string `json:"consentType"` // "privacy_policy" | "tos"
	Version     string `json:"version"`
}
