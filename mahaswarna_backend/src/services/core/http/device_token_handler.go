package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/core/http/middleware"
	ch "github.com/mahaswarna/contracts/http"
)

type DeviceTokenHandler struct{
	registerUC   *application.RegisterDeviceTokenUseCase
	deregisterUC *application.DeregisterDeviceTokenUseCase
}

func NewDeviceTokenHandler(
	reg *application.RegisterDeviceTokenUseCase,
	dereg *application.DeregisterDeviceTokenUseCase,
) *DeviceTokenHandler {
	return &DeviceTokenHandler{registerUC: reg, deregisterUC: dereg}
}

func (h *DeviceTokenHandler) RegisterToken(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	var req ch.DeviceTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	if err := h.registerUC.Execute(r.Context(), application.DeviceTokenInput{UserID: userID, Token: req.Token, DeviceID: req.DeviceID, Platform: req.Platform}); err != nil {
		writeError(w, 500, "internal_error", ""); return
	}
	w.WriteHeader(201)
}

// DeregisterToken handles DELETE /engagement/device-token/{token}.
// Idempotent: returns 204 whether or not the token existed.
// Token is URL-path encoded by the Android client (net.URLEncoder.encode).
func (h *DeviceTokenHandler) DeregisterToken(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }

	token := chi.URLParam(r, "token")
	if token == "" { writeError(w, 400, "validation_error", "token path param required"); return }

	if err := h.deregisterUC.Execute(r.Context(), userID, token); err != nil {
		writeError(w, 500, "internal_error", ""); return
	}
	w.WriteHeader(204)
}
