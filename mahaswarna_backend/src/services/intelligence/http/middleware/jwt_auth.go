package middleware

// JWTAuth, JWTClaims, and context helpers (UserIDFromCtx, TierFromCtx,
// RegionFromCtx) are implemented in middleware.go.
//
// This file documents the JWT middleware contract for the intelligence service.
//
// # Usage
//
//	r.Use(mw.JWTAuth(pubKey))
//
// pubKey is loaded from JWT_PUBLIC_KEY_PATH at startup (see main.go).
// The middleware validates RS256 Bearer tokens issued by the core service.
//
// # Context values set after a successful check
//
//	middleware.UserIDFromCtx(ctx)  → (uuid.UUID, bool)
//	middleware.TierFromCtx(ctx)    → "FREE" | "PREMIUM" | "ADMIN"
//	middleware.RegionFromCtx(ctx)  → city/region string from JWT claims
//
// # PREMIUM gating note
//
// Handlers must NOT gate access solely on TierFromCtx — the JWT claim may lag
// a cancellation by up to accessTTL (15 min). Use-cases re-check via
// SubscriptionProjection (Redis read model) for all PREMIUM-only operations.
