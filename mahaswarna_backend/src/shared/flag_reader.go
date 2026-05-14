package shared

import "context"

// FlagReader is the canonical read-only interface for feature flags.
// Both core and pricing implement this; callers that only need reads
// should depend on FlagReader rather than a concrete *FlagsRepository.
//
// Long-term: pricing's OQ-8 emergency write path (SetFlag) should be
// replaced by a call to POST /internal/flags/{key} on the core service,
// eliminating direct cross-schema writes. Until then, SetFlag remains
// in pricing's concrete FlagsRepository only.
type FlagReader interface {
	// GetFloat reads a float64 feature flag value.
	// Returns defaultVal when the key is absent or non-numeric.
	GetFloat(ctx context.Context, key string, defaultVal float64) float64
}
