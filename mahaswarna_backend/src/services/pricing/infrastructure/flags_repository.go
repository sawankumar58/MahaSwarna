package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagsRepository reads and writes feature flags from the feature_flags table.
// Satisfies both application.FlagReader and application.FlagWriter interfaces.
//
// ARCHITECTURE INVARIANT (OQ-8 gate):
//   The kill-switch escalation path in generate_ai_rates_usecase.go requires
//   that SetFlag("rate_limit_bff_free_rpm", "60") is committed and visible to
//   the gateway BEFORE SetFlag("kill_switch_ws", "true") is called.
//   The 5-second sleep between the two calls is the Redis TTL grace period.
//   This repository writes directly to the DB; gateway reads via Redis cache
//   (TTL ~5s as configured in core/infrastructure/redis_flags_cache.go).
//
// CROSS-SCHEMA NOTE: feature_flags lives in the core schema. Pricing accesses
// it directly only for the OQ-8 kill-switch path — an emergency write that
// cannot wait for a cross-service API call during a full Gemini outage.
// All other feature flag reads go through the standard FlagReader.GetFloat path.
type FlagsRepository struct {
	db *pgxpool.Pool
}

func NewFlagsRepository(db *pgxpool.Pool) *FlagsRepository {
	return &FlagsRepository{db: db}
}

// GetFloat reads a float64 feature flag value from the DB.
// Returns defaultVal when the key is absent or its value is not parseable.
func (r *FlagsRepository) GetFloat(ctx context.Context, key string, defaultVal float64) float64 {
	var val string
	err := r.db.QueryRow(ctx,
		`SELECT value FROM feature_flags WHERE key = $1`, key,
	).Scan(&val)
	if err != nil {
		// Key absent or DB error — return default without logging (called on hot path).
		return defaultVal
	}

	var f float64
	if _, err := fmt.Sscanf(val, "%f", &f); err != nil {
		return defaultVal
	}
	return f
}

// SetFlag writes or updates a feature flag value in the DB using UPSERT.
// The gateway's Redis flag cache will pick up the change within its TTL (~5s).
// Called exclusively by the OQ-8 kill-switch escalation path.
func (r *FlagsRepository) SetFlag(ctx context.Context, key, value string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO feature_flags (key, value)
		 VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("flags_repository set %s: %w", key, err)
	}
	return nil
}
