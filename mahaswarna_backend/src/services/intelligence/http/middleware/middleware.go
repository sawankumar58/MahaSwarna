package middleware

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type contextKey int

const (
	ctxUserID contextKey = iota
	ctxTier
	ctxRegion
)

// JWTClaims mirrors the claims issued by the core service.
type JWTClaims struct {
	jwt.RegisteredClaims
	Tier   string `json:"tier"`
	Region string `json:"region"`
}

// JWTAuth validates the Bearer token signed by core's RSA private key.
// The public key is loaded from JWT_PUBLIC_KEY_PATH at startup.
func JWTAuth(pubKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if raw == "" {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			var claims JWTClaims
			tok, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return pubKey, nil
			})
			if err != nil || !tok.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, "invalid subject", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			ctx = context.WithValue(ctx, ctxTier, claims.Tier)
			ctx = context.WithValue(ctx, ctxRegion, claims.Region)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromCtx extracts the authenticated user's UUID from the request context.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxUserID).(uuid.UUID)
	return v, ok
}

// TierFromCtx extracts the user's subscription tier from the request context.
func TierFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxTier).(string)
	return v
}

// RegionFromCtx extracts the user's region from the JWT claims stored in context.
// Returns an empty string if not set.
func RegionFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxRegion).(string)
	return v
}

// ───────────────────────────────────────────────────────────────────────────────

const serviceTokenHeader = "X-Service-Token"

// ServiceAuth validates the internal service-to-service token (HMAC secret).
// Used on internal-only endpoints not exposed through the gateway.
func ServiceAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := r.Header.Get(serviceTokenHeader)
			if tok != secret || secret == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ───────────────────────────────────────────────────────────────────────────────

const (
	defaultRateLimit = 60 // requests per window
	rateWindow       = time.Minute
)

// RateLimiter is a per-user sliding window rate limiter backed by Redis.
// Key schema: rl:{endpoint}:{userID} → request count
func RateLimiter(rdb *redis.Client, endpoint string, limit int) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = defaultRateLimit
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromCtx(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("rl:%s:%s", endpoint, userID)
			count, err := rdb.Incr(r.Context(), key).Result()
			if err == nil && count == 1 {
				rdb.Expire(r.Context(), key, rateWindow)
			}
			if count > int64(limit) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
