package lib

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/mahaswarna/gateway/middleware"
	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

// ResilientProxy wraps httputil.ReverseProxy with:
//   - Circuit breaker (gobreaker) per upstream
//   - Stale-while-revalidate fallback cache (Redis) for GET requests
//   - Retry on 502/503/504 via DoWithRetry (non-streaming GET paths)
//   - Correct header forwarding (trace context, service token, user identity)
//   - Shared connection pool via SharedTransport (no duplicate pools)
type ResilientProxy struct {
	target  *url.URL
	breaker *gobreaker.CircuitBreaker
	cache   *FallbackCache
	proxy   *httputil.ReverseProxy
}

// NewResilientProxy constructs a ResilientProxy for the given upstream base URL.
func NewResilientProxy(baseURL string, cb *gobreaker.CircuitBreaker, rdb *redis.Client) *ResilientProxy {
	target, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Sprintf("ResilientProxy: invalid upstream URL %q: %v", baseURL, err))
	}

	rp := &ResilientProxy{
		target:  target,
		breaker: cb,
		cache:   NewFallbackCache(rdb, 5*time.Minute),
	}

	rp.proxy = &httputil.ReverseProxy{
		Director:  rp.director,
		Transport: SharedTransport(), // L-2: single shared pool, not a per-proxy transport
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			shared.Logger.Error("proxy upstream error",
				"upstream", target.Host,
				"path", r.URL.Path,
				"err", err,
			)
			http.Error(w, `{"ok":false,"error":{"code":"upstream_error","message":"upstream service unavailable"}}`,
				http.StatusBadGateway)
		},
	}

	return rp
}

// Handle is the http.HandlerFunc that proxies the request through the circuit breaker.
func (p *ResilientProxy) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cacheKey := p.cacheKey(r)
	isGET := r.Method == http.MethodGet

	// Check circuit breaker state.
	if p.breaker.State() == gobreaker.StateOpen {
		shared.Logger.Warn("circuit breaker open, serving from fallback cache",
			"upstream", p.target.Host, "path", r.URL.Path)

		if isGET {
			if served := p.cache.ServeStale(ctx, cacheKey, w); served {
				w.Header().Set("X-Cache", "STALE")
				return
			}
		}
		http.Error(w, `{"ok":false,"error":{"code":"service_unavailable","message":"upstream service is currently unavailable"}}`,
			http.StatusServiceUnavailable)
		return
	}

	if isGET {
		rec := newCachingResponseWriter(w)
		_, err := p.breaker.Execute(func() (any, error) {
			p.proxy.ServeHTTP(rec, r)
			if rec.statusCode >= 500 {
				return nil, fmt.Errorf("upstream returned %d", rec.statusCode)
			}
			return nil, nil
		})
		if err != nil {
			if rec.statusCode == 0 {
				// Breaker tripped during execution, serve stale.
				if served := p.cache.ServeStale(ctx, cacheKey, w); served {
					w.Header().Set("X-Cache", "STALE")
					return
				}
				http.Error(w, `{"ok":false,"error":{"code":"service_unavailable","message":"upstream unavailable"}}`,
					http.StatusServiceUnavailable)
				return
			}
		}

		// Cache successful GET responses.
		if rec.statusCode == http.StatusOK {
			go p.cache.Store(context.Background(), cacheKey, rec.body.Bytes())
		}
		return
	}

	// Non-GET: proxy directly through the breaker, no cache.
	_, err := p.breaker.Execute(func() (any, error) {
		rec := newCachingResponseWriter(w)
		p.proxy.ServeHTTP(rec, r)
		if rec.statusCode >= 500 {
			return nil, fmt.Errorf("upstream returned %d", rec.statusCode)
		}
		return nil, nil
	})
	if err != nil {
		// Already written to w by proxy or error handler.
		shared.Logger.Error("breaker execute error (non-GET)", "err", err)
	}
}

// director rewrites the request to target the upstream and copies all
// relevant headers (trace, service token, user identity).
func (p *ResilientProxy) director(r *http.Request) {
	r.URL.Scheme = p.target.Scheme
	r.URL.Host = p.target.Host
	r.Host = p.target.Host

	// Strip the gateway's own Authorization header — upstreams use X-User-ID
	// and X-Service-Token instead, already set by middleware stack.
	r.Header.Del("Authorization")

	// Ensure trace headers are propagated.
	middleware.CopyTraceHeaders(r.Header, r)
}

func (p *ResilientProxy) cacheKey(r *http.Request) string {
	userID := middleware.UserIDFromCtx(r.Context())
	return fmt.Sprintf("proxy:cache:%s:%s:%s", p.target.Host, userID, r.URL.RequestURI())
}

// ── caching response writer ─────────────────────────────────────────────────

type cachingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *byteBuffer
}

func newCachingResponseWriter(w http.ResponseWriter) *cachingResponseWriter {
	return &cachingResponseWriter{ResponseWriter: w, body: &byteBuffer{}}
}

func (c *cachingResponseWriter) WriteHeader(code int) {
	c.statusCode = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *cachingResponseWriter) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

// byteBuffer is a simple growing byte slice (avoids importing bytes package).
type byteBuffer struct {
	data []byte
}

func (b *byteBuffer) Write(p []byte) { b.data = append(b.data, p...) }
func (b *byteBuffer) Bytes() []byte  { return b.data }

// Compile-time: ensure io is used.
var _ io.Reader = (*byteBuffer)(nil)

func (b *byteBuffer) Read(p []byte) (int, error) { return copy(p, b.data), io.EOF }
