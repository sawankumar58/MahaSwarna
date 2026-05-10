package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/application"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/http/middleware"
)

// CatalogHandler serves catalog search and recommendation endpoints.
type CatalogHandler struct {
	search    *application.SearchDesignUseCase
	recommend *application.RecommendDesignUseCase
}

func NewCatalogHandler(
	search *application.SearchDesignUseCase,
	recommend *application.RecommendDesignUseCase,
) *CatalogHandler {
	return &CatalogHandler{search: search, recommend: recommend}
}

// GET /v1/catalog/search?q=&region=&metal=&page=&limit=
// Note: `metal` (gold|silver|both) is an extension beyond the base OpenAPI spec;
// it is an additive, non-breaking filter and is safe to accept from clients.
func (h *CatalogHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	// Spec uses "limit" for page size; previously read "size" in error.
	limit, _ := strconv.Atoi(q.Get("limit"))

	result, err := h.search.Search(r.Context(), domain.SearchParams{
		Query:     q.Get("q"),
		Region:    q.Get("region"),
		MetalType: domain.MetalType(q.Get("metal")),
		Page:      page,
		PageSize:  limit,
	})
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GET /v1/catalog/designs/{id}
func (h *CatalogHandler) GetDesign(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid design id", http.StatusBadRequest)
		return
	}

	design, err := h.search.GetAndTrackView(r.Context(), id)
	if err != nil {
		http.Error(w, "design not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, design)
}

// GET /v1/catalog/recommendations?region=&metal=&limit=
func (h *CatalogHandler) Recommend(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))

	// Region falls back to the user's JWT region claim when not supplied as a
	// query param. middleware.RegionFromCtx extracts it from the verified token.
	region := q.Get("region")
	if region == "" {
		region = middleware.RegionFromCtx(r.Context())
	}

	designs, err := h.recommend.Recommend(r.Context(), application.RecommendInput{
		Region:    region,
		MetalType: domain.MetalType(q.Get("metal")),
		Limit:     limit,
	})
	if err != nil {
		http.Error(w, "recommend failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"designs": designs})
}
