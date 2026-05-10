package middleware

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type ctxClaimsKey struct{}

type Claims struct {
	jwt.RegisteredClaims
	Tier   string `json:"tier"`
	Region string `json:"region"`
}

var pubKeys []*rsa.PublicKey

func init() {
	if pem := os.Getenv("JWT_PUBLIC_KEY"); pem != "" {
		if k, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pem)); err == nil {
			pubKeys = append(pubKeys, k)
		}
	}
}

func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" { http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized); return }
		claims, err := parseToken(token)
		if err != nil { http.Error(w, `{"error":"token_expired"}`, http.StatusUnauthorized); return }
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxClaimsKey{}, claims)))
	})
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ctxClaimsKey{}).(*Claims)
	return c, ok
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	c, ok := ClaimsFromContext(ctx)
	if !ok { return uuid.UUID{}, fmt.Errorf("no claims") }
	return uuid.Parse(c.Subject)
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") { return strings.TrimPrefix(auth, "Bearer ") }
	return ""
}

func parseToken(s string) (*Claims, error) {
	var lastErr error
	for _, key := range pubKeys {
		k := key
		claims := &Claims{}
		_, err := jwt.ParseWithClaims(s, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok { return nil, fmt.Errorf("unexpected method") }
			return k, nil
		})
		if err == nil { return claims, nil }
		lastErr = err
	}
	return nil, lastErr
}
