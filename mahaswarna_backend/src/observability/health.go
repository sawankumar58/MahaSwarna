package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthChecker aggregates named readiness checks.
// GET /health      → always 200 (liveness — process is up)
// GET /health/ready → 200 only when all registered checks pass
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]CheckFn
}

// CheckFn is called on every /health/ready request. Return non-nil to signal unhealthy.
type CheckFn func(ctx context.Context) error

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{checks: make(map[string]CheckFn)}
}

// Register adds a named check.
func (h *HealthChecker) Register(name string, fn CheckFn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = fn
}

// LivenessHandler returns HTTP 200 unconditionally.
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ReadinessHandler runs all checks; returns 503 if any fail.
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		h.mu.RLock()
		checks := make(map[string]CheckFn, len(h.checks))
		for k, v := range h.checks {
			checks[k] = v
		}
		h.mu.RUnlock()

		results := make(map[string]string, len(checks))
		allOK := true
		for name, fn := range checks {
			if err := fn(ctx); err != nil {
				results[name] = err.Error()
				allOK = false
			} else {
				results[name] = "ok"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if !allOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(map[string]any{"status": map[bool]string{true: "ready", false: "not_ready"}[allOK], "checks": results})
	}
}
