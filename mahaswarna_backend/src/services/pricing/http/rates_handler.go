package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	contractshttp "github.com/mahaswarna/contracts/http"
	"github.com/mahaswarna/pricing/application"
	"github.com/mahaswarna/pricing/domain"
	"github.com/mahaswarna/shared/types"
)

// RatesHandler serves gold/silver rate HTTP endpoints.
type RatesHandler struct {
	getRateUC    *application.GetRateUseCase
	getHistoryUC *application.GetHistoryUseCase
}

func NewRatesHandler(getRateUC *application.GetRateUseCase, getHistoryUC *application.GetHistoryUseCase) *RatesHandler {
	return &RatesHandler{getRateUC: getRateUC, getHistoryUC: getHistoryUC}
}

// GetRate handles GET /rates/{cityID}
// Response: APIResponse[AIRateResponse]
func (h *RatesHandler) GetRate(w http.ResponseWriter, r *http.Request) {
	cityID := chi.URLParam(r, "cityID")
	if cityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_city", "cityID is required")
		return
	}

	snap, err := h.getRateUC.Execute(r.Context(), cityID)
	if err != nil {
		if errors.Is(err, application.ErrRateNotAvailable) {
			// Cold-start edge case: no snapshot exists anywhere.
			// ARCHITECTURE NOTE: 404, not 503. Client shows "Rates not available yet".
			writeError(w, http.StatusNotFound, "city_rates_not_available",
				"Rates not yet available for this city")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch rate")
		return
	}

	writeJSON(w, http.StatusOK, types.Success(toAIRateResponse(snap)))
}

// rateHistoryPoint is the per-snapshot shape for the history endpoint.
// Defined locally to guarantee it carries all required OpenAPI fields:
//
//	gold, silver, source, generatedAt, stale  (RateHistoryPoint schema)
//
// A local type is used instead of contracts/http.RateDataPoint to prevent
// schema drift if the shared contract changes upstream.
type rateHistoryPoint struct {
	Gold        float64 `json:"gold"`
	Silver      float64 `json:"silver"`
	Source      string  `json:"source"`
	GeneratedAt any     `json:"generatedAt"`
	Stale       bool    `json:"stale"`
}

// rateHistoryResponse wraps the history point list (mirrors RateHistoryResponse contract).
type rateHistoryResponse struct {
	CityID  string             `json:"cityId"`
	History []rateHistoryPoint `json:"history"`
}

// GetHistory handles GET /rates/{cityID}/history?limit=24
// Response: APIResponse[rateHistoryResponse]
func (h *RatesHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	cityID := chi.URLParam(r, "cityID")
	if cityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_city", "cityID is required")
		return
	}

	limit := 24
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil && n > 0 {
			limit = n
		}
	}

	snaps, err := h.getHistoryUC.Execute(r.Context(), cityID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch history")
		return
	}

	// Map all fields required by the OpenAPI RateHistoryPoint schema:
	// gold, silver, source, generatedAt, stale.
	// The Android RateHistoryScreen uses source for analytics and stale for the banner.
	points := make([]rateHistoryPoint, len(snaps))
	for i, s := range snaps {
		source := string(s.Source)
		if s.IsStale {
			// Stale overrides source label — matches GET /rates/{cityID} behaviour.
			source = "stale"
		}
		points[i] = rateHistoryPoint{
			Gold:        s.Gold,
			Silver:      s.Silver,
			Source:      source,
			GeneratedAt: s.GeneratedAt,
			Stale:       s.IsStale,
		}
	}

	resp := rateHistoryResponse{
		CityID:  cityID,
		History: points,
	}
	writeJSON(w, http.StatusOK, types.Success(resp))
}

// helpers

func toAIRateResponse(snap *domain.AIRateSnapshot) contractshttp.AIRateResponse {
	source := string(snap.Source)
	if snap.IsStale {
		source = "stale"
	}
	return contractshttp.AIRateResponse{
		CityID:      snap.CityID,
		Gold:        snap.Gold,
		Silver:      snap.Silver,
		Stale:       snap.IsStale,
		Source:      source,
		GeneratedAt: snap.GeneratedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(types.Fail[any](code, message))
}
