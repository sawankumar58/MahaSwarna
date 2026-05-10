package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ch "github.com/mahaswarna/contracts/http"
	"github.com/mahaswarna/intelligence/application"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/http/middleware"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// InvoiceHandler serves invoice generation and history endpoints.
type InvoiceHandler struct {
	generate *application.GenerateInvoiceUseCase
	invoices *infrastructure.InvoiceRepository
	shops    *infrastructure.ShopRepository
	pricing  *infrastructure.PricingClient
}

func NewInvoiceHandler(
	generate *application.GenerateInvoiceUseCase,
	invoices *infrastructure.InvoiceRepository,
	shops *infrastructure.ShopRepository,
	pricing *infrastructure.PricingClient,
) *InvoiceHandler {
	return &InvoiceHandler{generate: generate, invoices: invoices, shops: shops, pricing: pricing}
}

// POST /v1/shops/{id}/invoices
// shopID is taken from the path parameter {id} per the OpenAPI spec.
// GenerateInvoiceRequest does NOT contain a shopId field.
func (h *InvoiceHandler) GenerateInvoice(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Shop is identified by the path param, not the request body.
	shopID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid shopId", http.StatusBadRequest)
		return
	}

	var req ch.GenerateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Map contract DTOs → domain objects.
	var items []domain.InvoiceLineItem
	for _, i := range req.Items {
		items = append(items, domain.InvoiceLineItem{
			Description:  i.Description,
			WeightGrams:  i.WeightGrams,
			Karat:        i.Karat,
			MakingCharge: i.MakingCharge,
		})
	}

	// Resolve payment mode — required field per spec (enum: cash, upi, card).
	paymentMode, err := resolvePaymentMode(req.PaymentMode)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid paymentMode: %s", req.PaymentMode), http.StatusBadRequest)
		return
	}

	// Fetch live rates from the pricing service (http://pricing:4002/internal/rates/{cityID}).
	// If the pricing service is unavailable AND the client has not supplied an override rate,
	// reject with 503 per spec ("rate_unavailable").
	// If an override is supplied the invoice can be generated without a live rate.
	liveRates, rateErr := h.pricing.FetchRates(r.Context(), req.CityID)
	if rateErr != nil {
		if req.GoldRateOverride <= 0 {
			// No override available — cannot safely produce an invoice.
			// Use writeJSON so Content-Type is application/json (http.Error forces text/plain).
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate_unavailable"})
			return
		}
		// Client supplied a rate override (live WS rate on device) — proceed with zeros
		// for the server-side field; overrides will replace them below.
		liveRates = &infrastructure.LiveRates{}
	}

	liveGold := liveRates.GoldPerGram
	liveSilver := liveRates.SilverPerGram
	rateSource := domain.RateSourceLive

	var goldOverride *float64
	if req.GoldRateOverride > 0 {
		goldOverride = &req.GoldRateOverride
		liveGold = req.GoldRateOverride
		rateSource = domain.RateSourceClientOverride
	}

	var silverOverride *float64
	if req.SilverRateOverride > 0 {
		silverOverride = &req.SilverRateOverride
		liveSilver = req.SilverRateOverride
	}

	out, err := h.generate.Execute(r.Context(), application.GenerateInvoiceInput{
		UserID:             userID,
		ShopID:             shopID,
		CustomerName:       req.CustomerName,
		Items:              items,
		PaymentMode:        paymentMode,
		GoldRateOverride:   goldOverride,
		SilverRateOverride: silverOverride,
		LiveGoldRate:       liveGold,
		LiveSilverRate:     liveSilver,
		RateSource:         rateSource,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotPremium{}):
			http.Error(w, "PREMIUM required", http.StatusForbidden)
		case errors.Is(err, domain.ErrDailyLimitExceeded{}):
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		default:
			http.Error(w, "invoice generation failed", http.StatusInternalServerError)
		}
		return
	}

	// ADR-001: Return PDF bytes (base64 via encoding/json) inline.
	// Spec defines the success response as HTTP 200 (not 201).
	writeJSON(w, http.StatusOK, ch.InvoiceResponse{
		InvoiceID:   out.InvoiceID.String(),
		PdfBytes:    out.PDFBytes,
		GeneratedAt: out.GeneratedAt.In(infrastructure.IST).Format(time.RFC3339),
		RateSource:  string(out.RateSource),
	})
}

// GET /v1/shops/{shopId}/invoices?limit=&before=
func (h *InvoiceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shopID, err := uuid.Parse(chi.URLParam(r, "shopId"))
	if err != nil {
		http.Error(w, "invalid shopId", http.StatusBadRequest)
		return
	}

	// Verify the authenticated user owns this shop before returning any data.
	shop, err := h.shops.GetByID(r.Context(), shopID)
	if err != nil || shop == nil || shop.UserID != userID {
		http.Error(w, "shop not found or access denied", http.StatusForbidden)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	var before *time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			before = &t
		}
	}

	invs, err := h.invoices.ListByShop(r.Context(), shopID, limit, before)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": invs})
}

// resolvePaymentMode maps the client-supplied string to a typed domain value.
func resolvePaymentMode(raw string) (domain.PaymentMode, error) {
	switch domain.PaymentMode(raw) {
	case domain.PaymentModeCash, domain.PaymentModeUPI, domain.PaymentModeCard:
		return domain.PaymentMode(raw), nil
	case "":
		return domain.PaymentModeCash, nil
	default:
		return "", fmt.Errorf("unknown payment mode %q", raw)
	}
}
