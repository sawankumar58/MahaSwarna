package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// poolConfigs defines per-service MaxConns as specified in ARCHITECTURE.md §PostgreSQL Connection Pool.
// CRITICAL: pgx defaults to 4 when MaxConns is unset — always set explicitly.
var poolConfigs = map[string]int32{
	"gateway":      5,
	"core":         20,
	"pricing":      15,
	"intelligence": 15,
}

// NewPool creates a pgxpool.Pool for the named service.
// Reads DATABASE_URL from the environment; panics at startup if absent (caught by mustEnv in main).
func NewPool(ctx context.Context, service string) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool parse config: %w", err)
	}

	maxConns, ok := poolConfigs[service]
	if !ok {
		maxConns = 10 // safe default for unknown services
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool new (%s): %w", service, err)
	}

	// Verify connectivity at startup.
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping (%s): %w", service, err)
	}

	return pool, nil
}
