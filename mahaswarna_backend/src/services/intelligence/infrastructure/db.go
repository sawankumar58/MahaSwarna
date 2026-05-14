package infrastructure

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/infrastructure/migrations"
	"github.com/mahaswarna/infrastructure/postgres"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// NewDB creates and returns a connection pool for the intelligence service database.
func NewDB(ctx context.Context) (*pgxpool.Pool, error) {
	return postgres.NewPool(ctx, "intelligence")
}

// RunMigrations applies all pending intelligence service migrations.
// Delegates to the shared migration runner in infrastructure/migrations.
//
// NOTE: This runner is for the intelligence service's own schema only.
// It assumes the core migrations (which define set_updated_at()) have already run.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return migrations.RunMigrations(ctx, pool, "intelligence_migrations", migrationFS, "migrations")
}
