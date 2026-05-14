// Package migrations provides a generic idempotent migration runner for
// MahaSwarna microservices.
//
// Each service that manages its own schema (currently intelligence) embeds
// its SQL files via go:embed and calls RunMigrations at startup.
package migrations

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies all *.up.sql files from the provided filesystem in
// lexicographic order. It is idempotent: already-applied migrations are tracked
// in a service-specific table (tableName) and skipped on re-run.
//
// Usage in intelligence/infrastructure/db.go:
//
//	//go:embed migrations/*.sql
//	var migrationFS embed.FS
//
//	func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
//	    return migrations.RunMigrations(ctx, pool, "intelligence_migrations", migrationFS, "migrations")
//	}
//
// Parameters:
//   - pool:      connected pgxpool.Pool for the service database
//   - tableName: tracking table name (e.g. "intelligence_migrations") — one per service
//   - fsys:      embed.FS (or any fs.FS) containing the SQL files
//   - dir:       directory path within fsys to scan for *.up.sql files
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, tableName string, fsys fs.FS, dir string) error {
	// Ensure the migrations tracking table exists.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, tableName)); err != nil {
		return fmt.Errorf("create migrations table %s: %w", tableName, err)
	}

	// Collect migration filenames.
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read migration dir %s: %w", dir, err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		var count int
		if err := pool.QueryRow(ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE filename = $1`, tableName), name,
		).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue // already applied
		}

		sql, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s (filename) VALUES ($1)`, tableName), name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}
