package http

import (
	"encoding/json"
	"net/http"

	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/core/http/middleware"
	ch "github.com/mahaswarna/contracts/http"
)

type DeviceTokenHandler struct{ usecase *application.RegisterDeviceTokenUseCase }

func NewDeviceTokenHandler(uc *application.RegisterDeviceTokenUseCase) *DeviceTokenHandler { return &DeviceTokenHandler{usecase: uc} }

func (h *DeviceTokenHandler) RegisterToken(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	var req ch.DeviceTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	if err := h.usecase.Execute(r.Context(), application.DeviceTokenInput{UserID: userID, Token: req.Token, DeviceID: req.DeviceID, Platform: req.Platform}); err != nil {
		writeError(w, 500, "internal_error", ""); return
	}
	w.WriteHeader(201)
}
