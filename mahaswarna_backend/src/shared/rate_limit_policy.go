package shared

// RateLimitPolicy holds per-tier RPM limits for a given endpoint class.
// Values are read from feature flags at startup and refreshed on flag_updated events.
type RateLimitPolicy struct {
	FreeRPM    int
	PremiumRPM int
	AdminRPM   int
}

// DefaultBFFPolicy matches the architecture doc (§ BFF /bff/home).
var DefaultBFFPolicy = RateLimitPolicy{
	FreeRPM:    20,
	PremiumRPM: 60,
	AdminRPM:   600,
}

// DefaultGlobalPolicy is applied at the gateway level before tier is known.
var DefaultGlobalPolicy = RateLimitPolicy{
	FreeRPM:    120,
	PremiumRPM: 300,
	AdminRPM:   1200,
}
