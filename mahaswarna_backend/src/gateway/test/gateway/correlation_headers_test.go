package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mahaswarna/gateway/middleware"
)

func TestRequestID_GeneratedWhenAbsent(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			t.Error("expected X-Request-ID to be set on request")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID on response header")
	}
}

func TestRequestID_PreservesClientSuppliedID(t *testing.T) {
	const clientID = "my-correlation-id-123"

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != clientID {
			t.Errorf("expected request header %q, got %q", clientID, got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", clientID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Request-ID"); got != clientID {
		t.Errorf("expected response header %q, got %q", clientID, got)
	}
}

func TestTraceContext_InjectsSyntheticTraceparent(t *testing.T) {
	handler := middleware.TraceContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tp := r.Header.Get("Traceparent")
		if tp == "" {
			t.Error("expected Traceparent header to be injected")
		}
		// W3C traceparent format: 00-<traceId>-<spanId>-<flags>
		if len(tp) < 20 {
			t.Errorf("traceparent %q looks too short", tp)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestVersionValidator_RejectsOldVersion(t *testing.T) {
	t.Setenv("MIN_APP_VERSION", "2.0.0")

	handler := middleware.VersionValidator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	req.Header.Set("X-App-Version", "1.9.5")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUpgradeRequired {
		t.Errorf("expected 426, got %d", rr.Code)
	}
}

func TestVersionValidator_AllowsCurrentVersion(t *testing.T) {
	t.Setenv("MIN_APP_VERSION", "2.0.0")

	handler := middleware.VersionValidator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	req.Header.Set("X-App-Version", "2.1.0")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestVersionValidator_AllowsMissingHeader(t *testing.T) {
	t.Setenv("MIN_APP_VERSION", "2.0.0")

	handler := middleware.VersionValidator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/rates", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Missing header must pass through — force-update is handled by flags.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for missing header, got %d", rr.Code)
	}
}
