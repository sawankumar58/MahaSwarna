package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mahaswarna/gateway/middleware"
	"github.com/mahaswarna/shared"
)

func TestGlobalRateLimiter_AllowsUnderLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	policy := shared.RateLimitPolicy{FreeRPM: 10, PremiumRPM: 30, AdminRPM: 100}
	handler := middleware.GlobalRateLimiter(rdb, policy)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	req.RemoteAddr = "1.2.3.4:0"

	for i := 0; i < 10; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

func TestGlobalRateLimiter_Blocks429OnExcess(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	policy := shared.RateLimitPolicy{FreeRPM: 5, PremiumRPM: 20, AdminRPM: 100}
	handler := middleware.GlobalRateLimiter(rdb, policy)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	req.RemoteAddr = "2.3.4.5:0"

	blocked := 0
	for i := 0; i < 10; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked++
			if rr.Header().Get("Retry-After") == "" {
				t.Error("expected Retry-After header on 429")
			}
		}
	}

	if blocked == 0 {
		t.Error("expected at least one 429 above rate limit")
	}
}

func TestGlobalRateLimiter_SeparatesPerIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	policy := shared.RateLimitPolicy{FreeRPM: 3, PremiumRPM: 10, AdminRPM: 50}
	handler := middleware.GlobalRateLimiter(rdb, policy)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Exhaust limit for IP A.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
		req.RemoteAddr = "10.0.0.1:0"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	// IP B should still be allowed.
	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	req.RemoteAddr = "10.0.0.2:0"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for IP B (independent bucket), got %d", rr.Code)
	}
}
