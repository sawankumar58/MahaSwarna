package http

type DeviceTokenRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId"`
	Platform string `json:"platform"` // "android"
}
