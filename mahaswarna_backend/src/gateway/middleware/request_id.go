package middleware

import (
	"net/http"

	"github.com/segmentio/ksuid"
)

const headerRequestID = "X-Request-ID"

// RequestID generates a KSUID for every inbound request and sets it on
// both the request header (forwarded to upstreams) and the response header
// (returned to the client for correlation).
//
// If the client already supplies X-Request-ID it is preserved; a gateway-
// generated ID is only injected when the header is absent or empty.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(headerRequestID)
		if rid == "" {
			rid = ksuid.New().String()
			r.Header.Set(headerRequestID, rid)
		}
		w.Header().Set(headerRequestID, rid)
		next.ServeHTTP(w, r)
	})
}
