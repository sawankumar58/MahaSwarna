package http

import (
	"encoding/json"
	"net/http"

	"github.com/mahaswarna/core/application"
	ch "github.com/mahaswarna/contracts/http"
)

type AuthHandler struct {
	sendOTP *application.OTPSendUseCase
	login   *application.LoginUseCase
	refresh *application.RefreshUseCase
	logout  *application.LogoutUseCase
}

func NewAuthHandler(s *application.OTPSendUseCase, l *application.LoginUseCase, r *application.RefreshUseCase, lo *application.LogoutUseCase) *AuthHandler {
	return &AuthHandler{sendOTP: s, login: l, refresh: r, logout: lo}
}

func (h *AuthHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req ch.SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		writeError(w, 400, "validation_error", "phone is required"); return
	}
	out, err := h.sendOTP.Execute(r.Context(), application.OTPSendInput{Phone: req.Phone})
	if err != nil { mapError(w, err); return }
	writeJSON(w, 200, ch.SendOTPResponse{Provider: out.Provider})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req ch.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	out, err := h.login.Execute(r.Context(), application.LoginInput{
		Phone: req.Phone, FirebaseIDToken: req.FirebaseIDToken, OTP: req.OTP,
		IntegrityToken: req.IntegrityToken, CityID: req.CityID, Provider: req.Provider,
	})
	if err != nil { mapError(w, err); return }
	writeJSON(w, 200, ch.AuthTokenResponse{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, Tier: out.Tier})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req ch.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, 400, "validation_error", "refreshToken required"); return
	}
	out, err := h.refresh.Execute(r.Context(), req.RefreshToken)
	if err != nil { mapError(w, err); return }
	writeJSON(w, 200, ch.AuthTokenResponse{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, Tier: out.Tier})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req ch.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, 400, "validation_error", "refreshToken required"); return
	}
	if err := h.logout.Execute(r.Context(), req.RefreshToken); err != nil { mapError(w, err); return }
	w.WriteHeader(204)
}
