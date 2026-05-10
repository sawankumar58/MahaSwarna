package http

import (
	"encoding/json"
	"net/http"

	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/core/http/middleware"
	ch "github.com/mahaswarna/contracts/http"
)

type ComplianceHandler struct {
	deleteAccount *application.DeleteAccountUseCase
	logConsent    *application.LogConsentUseCase
}

func NewComplianceHandler(d *application.DeleteAccountUseCase, l *application.LogConsentUseCase) *ComplianceHandler {
	return &ComplianceHandler{deleteAccount: d, logConsent: l}
}

func (h *ComplianceHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	if err := h.deleteAccount.Execute(r.Context(), userID); err != nil { mapError(w, err); return }
	w.WriteHeader(204)
}

func (h *ComplianceHandler) LogConsent(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	var req ch.ConsentLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	isNew, err := h.logConsent.Execute(r.Context(), application.ConsentInput{UserID: userID, ConsentType: req.ConsentType, Version: req.Version})
	if err != nil { mapError(w, err); return }
	if isNew { w.WriteHeader(201) } else { w.WriteHeader(200) }
}
