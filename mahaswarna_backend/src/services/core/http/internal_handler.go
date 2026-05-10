package http

import (
	"net/http"
	"github.com/mahaswarna/core/infrastructure"
)

type InternalHandler struct{ subRepo *infrastructure.SubscriptionRepository }

func NewInternalHandler(s *infrastructure.SubscriptionRepository) *InternalHandler { return &InternalHandler{subRepo: s} }

func (h *InternalHandler) GetActiveSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.subRepo.ListAllActive(r.Context())
	if err != nil { writeError(w, 500, "internal_error", ""); return }
	type entry struct{ UserID string `json:"user_id"`; Tier string `json:"tier"` }
	result := make([]entry, 0, len(subs))
	for _, s := range subs {
		result = append(result, entry{UserID: s.UserID.String(), Tier: s.Tier})
	}
	writeJSON(w, 200, result)
}
