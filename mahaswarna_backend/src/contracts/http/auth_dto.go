package http

// SendOTPRequest / SendOTPResponse
type SendOTPRequest struct {
	Phone string `json:"phone"`
}
type SendOTPResponse struct {
	Provider string `json:"provider"` // "firebase" | "msg91"
}

// LoginRequest covers both Firebase-flow and MSG91-flow.
type LoginRequest struct {
	Phone           string `json:"phone"`
	FirebaseIDToken string `json:"firebaseIdToken,omitempty"`
	OTP             string `json:"otp,omitempty"`
	IntegrityToken  string `json:"integrityToken"`
	CityID          string `json:"cityId"`
	Provider        string `json:"provider"` // "firebase" | "msg91"
}

type AuthTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Tier         string `json:"tier"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}
