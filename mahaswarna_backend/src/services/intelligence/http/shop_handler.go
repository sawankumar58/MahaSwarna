package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ch "github.com/mahaswarna/contracts/http"
	"github.com/mahaswarna/intelligence/application"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/http/middleware"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// ShopHandler handles shop registration, banner upload, and shop query endpoints.
type ShopHandler struct {
	registerShop  *application.RegisterShopUseCase
	getBannerURL  *application.GetBannerUploadURLUseCase
	confirmBanner *application.ConfirmBannerUploadUseCase
	shops         *infrastructure.ShopRepository
}

func NewShopHandler(
	reg *application.RegisterShopUseCase,
	banner *application.GetBannerUploadURLUseCase,
	confirm *application.ConfirmBannerUploadUseCase,
	shops *infrastructure.ShopRepository,
) *ShopHandler {
	return &ShopHandler{
		registerShop:  reg,
		getBannerURL:  banner,
		confirmBanner: confirm,
		shops:         shops,
	}
}

// POST /v1/shops
func (h *ShopHandler) RegisterShop(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		GST     string `json:"gstNumber"`
		Phone   string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Address == "" || req.GST == "" || req.Phone == "" {
		http.Error(w, "name, address, gstNumber, phone are required", http.StatusBadRequest)
		return
	}

	shop, err := h.registerShop.Execute(r.Context(), application.RegisterShopInput{
		UserID:  userID,
		Name:    req.Name,
		Address: req.Address,
		GST:     req.GST,
		Phone:   req.Phone,
	})
	if err != nil {
		var notPremium domain.ErrNotPremium
		var alreadyExists domain.ErrShopAlreadyExists
		switch {
		case errors.As(err, &notPremium):
			http.Error(w, "PREMIUM subscription required", http.StatusForbidden)
		case errors.As(err, &alreadyExists):
			http.Error(w, "shop already registered", http.StatusConflict)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, shopToResponse(shop))
}

// GET /v1/shops — returns the authenticated user's shops as an array.
// Spec: response is array<ShopResponse> (not a single object).
func (h *ShopHandler) GetShop(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shop, err := h.shops.GetByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Return an empty array when the user has no shop — not a 404.
	// The spec defines the success shape as array<ShopResponse>.
	resp := []ch.ShopResponse{}
	if shop != nil {
		resp = append(resp, shopToResponse(shop))
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /v1/shops/{id}/banner — returns a presigned S3 PUT URL for the banner.
// The {id} path param identifies the shop; ownership is verified in the use case.
func (h *ShopHandler) GetBannerUploadURL(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate the shop ID from the path and assert ownership before presigning.
	shopID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid shopId", http.StatusBadRequest)
		return
	}
	shop, err := h.shops.GetByID(r.Context(), shopID)
	if err != nil || shop == nil || shop.UserID != userID {
		http.Error(w, "shop not found or access denied", http.StatusForbidden)
		return
	}

	out, err := h.getBannerURL.Execute(r.Context(), userID)
	if err != nil {
		var notPremium domain.ErrNotPremium
		if errors.As(err, &notPremium) {
			http.Error(w, "PREMIUM required", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"presignedUrl": out.PresignedURL,
		"objectKey":    out.ObjectKey,
	})
}

// POST /v1/shops/{id}/banner/confirm — confirms the S3 upload and triggers moderation.
// The {id} path param is authoritative for shop ownership; body shopId is ignored.
func (h *ShopHandler) ConfirmBannerUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shopID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid shopId", http.StatusBadRequest)
		return
	}

	var req struct {
		ObjectKey string `json:"objectKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	shop, err := h.confirmBanner.Execute(r.Context(), application.ConfirmBannerInput{
		UserID:    userID,
		ShopID:    shopID,
		ObjectKey: req.ObjectKey,
		S3Bucket:  os.Getenv("S3_BUCKET"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	writeJSON(w, http.StatusOK, shopToResponse(shop))
}

// shopToResponse converts a domain Shop into the full OpenAPI ShopResponse DTO.
// All required fields (address, gstNumber, phone, userId, createdAt) are populated.
func shopToResponse(shop *domain.Shop) ch.ShopResponse {
	resp := ch.ShopResponse{
		ID:        shop.ID.String(),
		Name:      shop.Name,
		Address:   shop.Address,
		GSTNumber: shop.GSTNumber,
		Phone:     shop.Phone,
		UserID:    shop.UserID.String(),
		CreatedAt: shop.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if shop.BannerURL != nil {
		resp.BannerURL = *shop.BannerURL
	}
	return resp
}
