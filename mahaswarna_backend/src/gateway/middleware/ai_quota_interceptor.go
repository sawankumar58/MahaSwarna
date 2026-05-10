package middleware

import (
	"net/http"
)

// AIQuotaInterceptor is a response-side middleware applied to all /v1/catalog/* routes
// that may invoke Gemini AI on the intelligence service.
//
// Request side:
//   - Checks the "ai" kill-switch (loaded by FeatureFlags); returns 503 if active.
//   - Validates that a userID is present (must run after JWTPreValidator).
//
// Response side:
//   - The intelligence service sets X-Internal-Ai-Quota-* headers on its HTTP
//     responses after every Gemini call:
//       X-Internal-Ai-Quota-Used      <integer>  (calls used this IST day)
//       X-Internal-Ai-Quota-Limit     <integer>  (max calls per tier per day)
//       X-Internal-Ai-Quota-Reset-At  <unix_epoch_seconds> (next IST midnight)
//   - This middleware maps those internal headers to the client-facing contract:
//       X-Ai-Quota-Used      → forwarded to Android AiQuotaInterceptor.kt
//       X-Ai-Quota-Limit     → written to PreferenceStore.aiQuotaLimit
//       X-Ai-Quota-Reset-At  → written to PreferenceStore.aiQuotaResetAt
//   - Internal headers are stripped from the outbound response.
//   - On routes that do NOT call Gemini, no internal headers are set; the client
//     retains its last-known quota values from PreferenceStore.
//
// Must be applied AFTER JWTPreValidator and FeatureFlags.
func AIQuotaInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Kill-switch check — flags already loaded by FeatureFlags middleware.
		if IsKillSwitchActive(ctx, "ai") {
			writeError(w, http.StatusServiceUnavailable, "ai_disabled",
				"AI features are temporarily disabled")
			return
		}

		userID := UserIDFromCtx(ctx)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "authentication required")
			return
		}

		// Wrap the response writer so we can intercept X-Internal-Ai-Quota-*
		// headers written by the intelligence service before they reach the client.
		rec := newQuotaRelayWriter(w)
		next.ServeHTTP(rec, r)
		// quotaRelayWriter.WriteHeader already performed the header mapping;
		// no further action required here.
	})
}

// quotaRelayWriter intercepts upstream X-Internal-Ai-Quota-* response headers
// and remaps them to the client-facing X-Ai-Quota-* contract before the
// response headers are flushed to the network.
//
// The httputil.ReverseProxy sets all response headers on w.Header() then calls
// w.WriteHeader(statusCode). We intercept WriteHeader to perform the mapping
// atomically before headers are sent.
type quotaRelayWriter struct {
	http.ResponseWriter
	headersDone bool
}

func newQuotaRelayWriter(w http.ResponseWriter) *quotaRelayWriter {
	return &quotaRelayWriter{ResponseWriter: w}
}

// WriteHeader performs the internal → client header mapping immediately before
// the HTTP status line and headers are written to the wire.
func (q *quotaRelayWriter) WriteHeader(code int) {
	if !q.headersDone {
		q.headersDone = true
		h := q.ResponseWriter.Header()

		if v := h.Get("X-Internal-Ai-Quota-Used"); v != "" {
			h.Set("X-Ai-Quota-Used", v)
			h.Del("X-Internal-Ai-Quota-Used")
		}
		if v := h.Get("X-Internal-Ai-Quota-Limit"); v != "" {
			h.Set("X-Ai-Quota-Limit", v)
			h.Del("X-Internal-Ai-Quota-Limit")
		}
		if v := h.Get("X-Internal-Ai-Quota-Reset-At"); v != "" {
			h.Set("X-Ai-Quota-Reset-At", v)
			h.Del("X-Internal-Ai-Quota-Reset-At")
		}
	}
	q.ResponseWriter.WriteHeader(code)
}

// Write delegates to the underlying ResponseWriter. If the handler writes a
// body without calling WriteHeader first (implicit 200), the mapping must still
// run. We trigger it here to handle that case.
func (q *quotaRelayWriter) Write(b []byte) (int, error) {
	if !q.headersDone {
		// Trigger header mapping before the implicit WriteHeader(200).
		q.WriteHeader(http.StatusOK)
	}
	return q.ResponseWriter.Write(b)
}
