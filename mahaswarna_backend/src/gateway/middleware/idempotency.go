package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
)

const (
	headerIdempotencyKey = "Idempotency-Key"
	idempotencyTTL       = 24 * time.Hour
)

// idempotencyCacheEntry is stored in Redis to replay prior responses.
type idempotencyCacheEntry struct {
	StatusCode int             `json:"statusCode"`
	Body       json.RawMessage `json:"body"`
}

// Idempotency deduplicates non-GET/HEAD requests using the Idempotency-Key header.
// If the key has been seen within 24 hours the cached response is replayed
// (same status code + body) without forwarding to the upstream.
//
// The Idempotency-Key header is OPTIONAL for clients. If absent the request
// is forwarded normally without any dedup checks.
//
// Key format (authenticated): idempotency:user:{userID}:{idempotency-key}
// Key format (unauthenticated, e.g. /auth/* routes): idempotency:anon:{remoteAddr}:{idempotency-key}
//
// The namespace distinction prevents cross-user key collisions on auth routes
// where JWTPreValidator has not yet run and UserIDFromCtx returns "".
func Idempotency(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only idempotency-check mutating methods; GET/HEAD are naturally idempotent.
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			ikey := r.Header.Get(headerIdempotencyKey)
			if ikey == "" {
				next.ServeHTTP(w, r)
				return
			}

			redisKey := idempotencyRedisKey(r)
			ctx := r.Context()

			// Check cache hit.
			cached, err := rdb.Get(ctx, redisKey).Bytes()
			if err == nil && len(cached) > 0 {
				var entry idempotencyCacheEntry
				if json.Unmarshal(cached, &entry) == nil {
					w.Header().Set("X-Idempotency-Replayed", "true")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(entry.StatusCode)
					_, _ = w.Write(entry.Body)
					return
				}
			}

			// Capture upstream response for caching.
			rec := &responseRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Only cache successful (2xx) or idempotent error (422, 409) responses.
			// 5xx errors are transient and should not be cached.
			if rec.code < 500 {
				entry := idempotencyCacheEntry{
					StatusCode: rec.code,
					Body:       rec.body.Bytes(),
				}
				data, merr := json.Marshal(entry)
				if merr == nil {
					if serr := rdb.Set(ctx, redisKey, data, idempotencyTTL).Err(); serr != nil {
						shared.Logger.Warn("idempotency: failed to cache response",
							"key", redisKey, "err", serr)
					}
				}
			}
		})
	}
}

// idempotencyRedisKey builds a collision-safe Redis key.
// Authenticated requests are namespaced by userID; unauthenticated requests
// (e.g. POST /auth/login before JWTPreValidator runs) are namespaced by the
// client's remote address to prevent cross-user key collisions.
func idempotencyRedisKey(r *http.Request) string {
	ikey := r.Header.Get(headerIdempotencyKey)
	if userID := UserIDFromCtx(r.Context()); userID != "" {
		return fmt.Sprintf("idempotency:user:%s:%s", userID, ikey)
	}
	// Unauthenticated: scope by remote address so two clients with the same
	// Idempotency-Key value do not collide on /auth/* endpoints.
	return fmt.Sprintf("idempotency:anon:%s:%s", r.RemoteAddr, ikey)
}

// responseRecorder captures status code and body from the next handler.
type responseRecorder struct {
	http.ResponseWriter
	code int
	body bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Compile-time: ensure io is used (body reading in proxy will use it).
var _ io.Reader = (*bytes.Buffer)(nil)
