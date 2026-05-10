package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/http/middleware"
	"github.com/mahaswarna/core/infrastructure"
	ch "github.com/mahaswarna/contracts/http"
)

type AlertsHandler struct{ repo *infrastructure.AlertsRepository }

func NewAlertsHandler(r *infrastructure.AlertsRepository) *AlertsHandler { return &AlertsHandler{repo: r} }

func (h *AlertsHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	alerts, err := h.repo.ListByUser(r.Context(), userID)
	if err != nil { writeError(w, 500, "internal_error", ""); return }
	resp := ch.AlertListResponse{Alerts: make([]ch.AlertResponse, 0, len(alerts))}
	for _, a := range alerts {
		resp.Alerts = append(resp.Alerts, ch.AlertResponse{ID: a.ID.String(), CityID: a.CityID, Metal: a.Metal, Threshold: a.Threshold, Direction: a.Direction, CreatedAt: a.CreatedAt, DeliveredAt: a.DeliveredAt})
	}
	writeJSON(w, 200, resp)
}

func (h *AlertsHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	var req ch.CreateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "validation_error", err.Error()); return
	}
	alert, err := h.repo.Create(r.Context(), domain.Alert{UserID: userID, CityID: req.CityID, Metal: req.Metal, Threshold: req.Threshold, Direction: req.Direction})
	if err != nil { writeError(w, 500, "internal_error", ""); return }
	writeJSON(w, 201, ch.AlertResponse{ID: alert.ID.String(), CityID: alert.CityID, Metal: alert.Metal, Threshold: alert.Threshold, Direction: alert.Direction, CreatedAt: alert.CreatedAt})
}

func (h *AlertsHandler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil { writeError(w, 401, "unauthorized", ""); return }
	alertID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil { writeError(w, 400, "validation_error", "invalid id"); return }
	if err := h.repo.Delete(r.Context(), alertID, userID); err != nil {
		writeError(w, 404, "not_found", ""); return
	}
	w.WriteHeader(204)
}
