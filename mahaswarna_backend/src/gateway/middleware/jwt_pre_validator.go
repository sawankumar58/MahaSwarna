package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mahaswarna/shared"
)

// ctxKey is an unexported type for context keys in the gateway package.
type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyTier
	ctxKeyPhone
)

// MahaSwarnaClaims are the custom JWT claims issued by the core service.
type MahaSwarnaClaims struct {
	UserID string `json:"sub"`
	Phone  string `json:"phone"`
	Tier   string `json:"tier"` // "free" | "premium" | "admin"
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

// JWTPreValidator validates the Authorization: Bearer <token> header using RS256.
// On success it injects UserID, Tier, and Phone into the request context
// so downstream handlers and upstreams don't need to re-parse the token.
//
// The public key is loaded once at middleware construction from JWT_PUBLIC_KEY
// (RSA public key in PEM format). The gateway only checks:
//   - Token is well-formed and signed with the RSA private key matching JWT_PUBLIC_KEY
//   - Token is not expired
//   - Required claims (sub, tier, jti) are present
//
// JTI revocation is enforced by the core service; the gateway does not maintain
// a revocation list to avoid the per-request Redis lookup on every authenticated call.
func JWTPreValidator(next http.Handler) http.Handler {
	pubKey := mustLoadRSAPublicKey(os.Getenv("JWT_PUBLIC_KEY"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization header is required")
			return
		}

		claims := &MahaSwarnaClaims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			// Enforce RS256 — reject HS256, none, and any other algorithm.
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		}, jwt.WithExpirationRequired())

		if err != nil || !token.Valid {
			code, msg := jwtErrorCode(err)
			writeError(w, http.StatusUnauthorized, code, msg)
			return
		}

		if claims.UserID == "" || claims.Tier == "" || claims.JTI == "" {
			writeError(w, http.StatusUnauthorized, "invalid_token", "token is missing required claims")
			return
		}

		// Inject claims into context for downstream middleware and handlers.
		ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxKeyTier, claims.Tier)
		ctx = context.WithValue(ctx, ctxKeyPhone, claims.Phone)

		// Forward user identity to upstreams as headers (avoids re-parsing JWT).
		r = r.WithContext(ctx)
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-User-Tier", claims.Tier)

		next.ServeHTTP(w, r)
	})
}

// UserIDFromCtx extracts the authenticated user ID from a request context.
func UserIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(string)
	return v
}

// TierFromCtx extracts the subscription tier from a request context.
func TierFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTier).(string)
	return v
}

// WithUser injects user identity into a context. Used in tests and internal
// code paths that bypass JWTPreValidator (e.g. synthetic BFF test requests).
func WithUser(ctx context.Context, userID, tier string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	return context.WithValue(ctx, ctxKeyTier, tier)
}

// mustLoadRSAPublicKey parses a PEM-encoded RSA public key from the given string.
// Panics at startup if the key is missing or malformed so misconfiguration is
// caught immediately rather than at the first authenticated request.
func mustLoadRSAPublicKey(pemStr string) *rsa.PublicKey {
	if pemStr == "" {
		panic("JWTPreValidator: JWT_PUBLIC_KEY is not set")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		panic("JWTPreValidator: JWT_PUBLIC_KEY is not valid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("JWTPreValidator: failed to parse JWT_PUBLIC_KEY: " + err.Error())
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		panic("JWTPreValidator: JWT_PUBLIC_KEY is not an RSA public key")
	}
	shared.Logger.Info("JWTPreValidator: RSA public key loaded",
		"key_size_bits", rsaPub.N.BitLen())
	return rsaPub
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func jwtErrorCode(err error) (code, msg string) {
	switch {
	case isExpired(err):
		return shared.ErrTokenExpired.Error(), "access token has expired"
	case err != nil:
		return shared.ErrUnauthorized.Error(), "token is invalid or malformed"
	default:
		return shared.ErrUnauthorized.Error(), "authentication failed"
	}
}

func isExpired(err error) bool {
	return err != nil && strings.Contains(err.Error(), "expired")
}

// Compile-time: ensure time is used (jwt exp validation uses it transitively).
var _ = time.Second
