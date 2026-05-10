package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mahaswarna/gateway/bff"
	"github.com/mahaswarna/gateway/lib"
	"github.com/mahaswarna/gateway/middleware"
	contractshttp "github.com/mahaswarna/contracts/http"
)

func TestBFFAggregator_SuccessPath(t *testing.T) {
	// Stub pricing service.
	pricing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/rates":
			writeJSON(w, map[string]any{
				"ok":   true,
				"data": contractshttp.RateResponse{CityID: "mumbai", Gold: 72000, Silver: 85000},
			})
		case "/internal/rates/ai":
			writeJSON(w, map[string]any{
				"ok":   true,
				"data": contractshttp.AIRateResponse{CityID: "mumbai", Gold: 72100, Silver: 85050, Source: "live"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer pricing.Close()

	// Stub core service (alerts).
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok":   true,
			"data": map[string]any{"alerts": []any{}},
		})
	}))
	defer core.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cb := lib.NewBreaker("test")
	agg := bff.NewHomeAggregator(
		core.URL, pricing.URL, pricing.URL, // intelligence unused in this test
		cb, cb, cb,
		rdb,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/bff/home?cityId=mumbai", nil)
	// L-4: inject user context via exported middleware.WithUser so
	// middleware.UserIDFromCtx returns a valid value in the aggregator.
	req = req.WithContext(middleware.WithUser(req.Context(), "user-123", "free"))
	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("X-User-Tier", "free")

	rr := httptest.NewRecorder()
	agg.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var envelope struct {
		OK   bool                         `json:"ok"`
		Data contractshttp.BFFHomeResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !envelope.OK {
		t.Error("expected ok=true")
	}
	if envelope.Data.Rate == nil {
		t.Error("expected rate to be populated")
	}
	if envelope.Data.Degraded {
		t.Error("expected _degraded=false on full success")
	}
}

func TestBFFAggregator_DegradedOnUpstreamFailure(t *testing.T) {
	// Pricing service always 500s.
	pricing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer pricing.Close()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "data": map[string]any{"alerts": []any{}}})
	}))
	defer core.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cb := lib.NewBreaker("test-degraded")

	agg := bff.NewHomeAggregator(core.URL, pricing.URL, pricing.URL, cb, cb, cb, rdb)

	req := httptest.NewRequest(http.MethodGet, "/v1/bff/home?cityId=mumbai", nil)
	// L-4: inject user context correctly.
	req = req.WithContext(middleware.WithUser(req.Context(), "user-456", "premium"))
	req.Header.Set("X-User-ID", "user-456")
	req.Header.Set("X-User-Tier", "premium")

	rr := httptest.NewRecorder()
	agg.Handle(rr, req)

	// Gateway should still return 200 with _degraded=true.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (degraded), got %d", rr.Code)
	}

	var envelope struct {
		OK   bool                         `json:"ok"`
		Data contractshttp.BFFHomeResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !envelope.Data.Degraded {
		t.Error("expected _degraded=true when pricing fails")
	}
}

func TestBFFAggregator_CacheKeyIsCityScoped(t *testing.T) {
	// Verify that the shared cache key does not include userID (city-scoped).
	// Two users in the same city should share one rate cache entry.
	pricing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/rates":
			writeJSON(w, map[string]any{
				"ok":   true,
				"data": contractshttp.RateResponse{CityID: "delhi", Gold: 71500, Silver: 84000},
			})
		case "/internal/rates/ai":
			writeJSON(w, map[string]any{
				"ok":   true,
				"data": contractshttp.AIRateResponse{CityID: "delhi", Gold: 71600, Silver: 84100, Source: "live"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer pricing.Close()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true, "data": map[string]any{"alerts": []any{}}})
	}))
	defer core.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cb := lib.NewBreaker("test-cache")

	agg := bff.NewHomeAggregator(core.URL, pricing.URL, pricing.URL, cb, cb, cb, rdb)

	// First user — populates cache.
	req1 := httptest.NewRequest(http.MethodGet, "/v1/bff/home?cityId=delhi", nil)
	req1 = req1.WithContext(middleware.WithUser(req1.Context(), "user-A", "free"))
	agg.Handle(httptest.NewRecorder(), req1)

	// Shared cache key must exist in Redis as home:shared:delhi (no userID).
	keys, err := rdb.Keys(req1.Context(), "home:shared:*").Result()
	if err != nil {
		t.Fatalf("redis keys error: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("expected home:shared:{cityID} key to exist in Redis after successful BFF response")
	}
	for _, k := range keys {
		if k != "home:shared:delhi" {
			t.Errorf("unexpected cache key %q; shared key must be city-scoped, not user-scoped", k)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
