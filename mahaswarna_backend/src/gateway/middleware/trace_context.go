package middleware

import (
	"net/http"

	"github.com/segmentio/ksuid"
)

// traceHeaders are forwarded verbatim to every upstream call.
// We support both W3C Trace Context (traceparent/tracestate) and
// legacy B3 single/multi headers so any tracing backend works.
var traceHeaders = []string{
	"Traceparent",
	"Tracestate",
	"X-B3-TraceId",
	"X-B3-SpanId",
	"X-B3-Sampled",
	"X-B3-ParentSpanId",
	"X-B3-Flags",
	"B3",
	"X-Request-ID",
}

// TraceContext ensures a traceparent header exists on every request.
// If neither traceparent nor X-B3-TraceId is present it generates a
// synthetic W3C traceparent so that the gateway always initiates a trace.
func TraceContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Traceparent") == "" && r.Header.Get("X-B3-TraceId") == "" {
			// Generate a minimal W3C traceparent: version=00, traceId=ksuid-hex, spanId=8-byte-ksuid-hex, flags=01
			traceID := ksuid.New().String()
			spanID := ksuid.New().String()
			if len(traceID) > 32 {
				traceID = traceID[:32]
			}
			if len(spanID) > 16 {
				spanID = spanID[:16]
			}
			r.Header.Set("Traceparent", "00-"+traceID+"-"+spanID+"-01")
		}
		next.ServeHTTP(w, r)
	})
}

// CopyTraceHeaders copies all recognised trace headers from src into dst.
// Used by lib.ResilientProxy when constructing upstream requests.
func CopyTraceHeaders(src http.Header, dst *http.Request) {
	for _, h := range traceHeaders {
		if v := src.Get(h); v != "" {
			dst.Header.Set(h, v)
		}
	}
}
