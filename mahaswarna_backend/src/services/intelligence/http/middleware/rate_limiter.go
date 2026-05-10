package middleware

// RateLimiter is implemented in middleware.go.
//
// This file documents the rate limiter contract for the intelligence service.
//
// # Signature
//
//	func RateLimiter(rdb *redis.Client, endpoint string, limit int) func(http.Handler) http.Handler
//
// # Usage in router.go
//
//	r.Use(mw.RateLimiter(rdb, "catalog", 120))  // 120 req/min per user
//	r.Use(mw.RateLimiter(rdb, "invoice", 10))   // 10 req/min per user
//
// # Key schema
//
//	rl:{endpoint}:{userID}  →  request count (TTL: 1 minute)
//
// # Behaviour
//
//   - Per-user sliding window (not per-IP). Requires JWTAuth to run first.
//   - Increments on every request; sets 1-minute TTL on first increment.
//   - Returns HTTP 429 with Retry-After: 60 when count exceeds limit.
//   - If no userID in context (auth middleware skipped), rate limiter is a no-op.
//   - limit ≤ 0 falls back to defaultRateLimit (60 req/min).
