package http

import (
	"encoding/json"
	"net/http"

	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/core/http/middleware"
	ch "github.com/mahaswarna/contracts/http"
)

type BillingHandler struct {
	verify  *application.VerifyReceiptUseCase
	restore *application.RestoreSubscriptionUseCase
}

func NewBillingHandler(v *application.VerifyReceiptUseCase, r *application.RestoreSubscriptionUseCase) *BillingHandler {
	return &BillingHandler{verify: v, restore: r}
}

func (h *BillingHandler) VerifyReceipt(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	var req ch.VerifyReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	out, err := h.verify.Execute(r.Context(), application.ReceiptInput{UserID: userID, PurchaseToken: req.PurchaseToken, ProductID: req.ProductID, PackageName: req.PackageName})
	if err != nil { mapError(w, err); return }
	writeJSON(w, 200, ch.BillingResponse{Tier: out.Tier, ExpiresAt: out.ExpiresAt})
}

func (h *BillingHandler) RestoreSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	out, err := h.restore.Execute(r.Context(), userID)
	if err != nil { mapError(w, err); return }
	writeJSON(w, 200, ch.BillingResponse{Tier: out.Tier, ExpiresAt: out.ExpiresAt})
}
