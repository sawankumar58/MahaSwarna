package http

import (
	"net/http"
	"github.com/mahaswarna/core/infrastructure"
)

type FlagsHandler struct{ repo *infrastructure.FlagsRepository }

func NewFlagsHandler(r *infrastructure.FlagsRepository) *FlagsHandler { return &FlagsHandler{repo: r} }

func (h *FlagsHandler) GetPublicFlags(w http.ResponseWriter, r *http.Request) {
	resp, err := h.repo.GetPublicResponse(r.Context())
	if err != nil { writeError(w, 500, "internal_error", ""); return }
	writeJSON(w, 200, resp)
}
