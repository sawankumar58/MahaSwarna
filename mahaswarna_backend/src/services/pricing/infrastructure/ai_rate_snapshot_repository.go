package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/pricing/domain"
)

// AIRateSnapshotRepository persists Gemini AI rate snapshots to PostgreSQL.
// The DB is a fallback when Redis is cold; Redis is the primary read path.
type AIRateSnapshotRepository struct {
	db *pgxpool.Pool
}

func NewAIRateSnapshotRepository(db *pgxpool.Pool) *AIRateSnapshotRepository {
	return &AIRateSnapshotRepository{db: db}
}

// InsertSnapshot writes a new snapshot row and relies on the pg trigger
// trg_rate_snapshot_notify to NOTIFY ai_rate_snapshot_ready.
func (r *AIRateSnapshotRepository) InsertSnapshot(ctx context.Context, snap *domain.AIRateSnapshot) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ai_rate_snapshots (city_id, gold, silver, source, is_stale, generated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		snap.CityID, snap.Gold, snap.Silver, string(snap.Source), snap.IsStale, snap.GeneratedAt,
	)
	if err != nil {
		return fmt.Errorf("insert snapshot %s: %w", snap.CityID, err)
	}
	return nil
}

// GetLatest returns the most recent snapshot for a city from the DB.
// Returns nil, nil if no snapshot exists (cold-start edge case).
func (r *AIRateSnapshotRepository) GetLatest(ctx context.Context, cityID string) (*domain.AIRateSnapshot, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, city_id, gold, silver, source, is_stale, generated_at, created_at
		 FROM ai_rate_snapshots
		 WHERE city_id = $1
		 ORDER BY generated_at DESC
		 LIMIT 1`,
		cityID,
	)
	return scanSnapshot(row)
}

// GetLatestAll returns the most recent snapshot for every city.
// Used by warmup_cache.sh and startup Redis warming.
func (r *AIRateSnapshotRepository) GetLatestAll(ctx context.Context) ([]*domain.AIRateSnapshot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT ON (city_id)
		   id, city_id, gold, silver, source, is_stale, generated_at, created_at
		 FROM ai_rate_snapshots
		 ORDER BY city_id, generated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest all: %w", err)
	}
	defer rows.Close()

	var snaps []*domain.AIRateSnapshot
	for rows.Next() {
		var s domain.AIRateSnapshot
		var src string
		if err := rows.Scan(&s.ID, &s.CityID, &s.Gold, &s.Silver, &src, &s.IsStale, &s.GeneratedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot row: %w", err)
		}
		s.Source = domain.RateSource(src)
		snaps = append(snaps, &s)
	}
	return snaps, rows.Err()
}

// GetHistory returns up to limit snapshots for a city, newest first.
func (r *AIRateSnapshotRepository) GetHistory(ctx context.Context, cityID string, limit int) ([]*domain.AIRateSnapshot, error) {
	if limit <= 0 || limit > 168 { // max 168 = 1 week of hourly snapshots
		limit = 24
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, city_id, gold, silver, source, is_stale, generated_at, created_at
		 FROM ai_rate_snapshots
		 WHERE city_id = $1
		 ORDER BY generated_at DESC
		 LIMIT $2`,
		cityID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get history %s: %w", cityID, err)
	}
	defer rows.Close()

	var snaps []*domain.AIRateSnapshot
	for rows.Next() {
		var s domain.AIRateSnapshot
		var src string
		if err := rows.Scan(&s.ID, &s.CityID, &s.Gold, &s.Silver, &src, &s.IsStale, &s.GeneratedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		s.Source = domain.RateSource(src)
		snaps = append(snaps, &s)
	}
	return snaps, rows.Err()
}

// MarkStale updates is_stale = true for the latest snapshot of a city.
func (r *AIRateSnapshotRepository) MarkStale(ctx context.Context, cityID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE ai_rate_snapshots SET is_stale = TRUE
		 WHERE id = (
		   SELECT id FROM ai_rate_snapshots
		   WHERE city_id = $1
		   ORDER BY generated_at DESC
		   LIMIT 1
		 )`,
		cityID,
	)
	return err
}

// Prune deletes snapshots older than 30 days. Called by cleanup_old_data.sh.
func (r *AIRateSnapshotRepository) Prune(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM ai_rate_snapshots WHERE created_at < $1`,
		time.Now().UTC().Add(-30*24*time.Hour),
	)
	if err != nil {
		return 0, fmt.Errorf("prune snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetActiveCities returns all city IDs that are active.
func (r *AIRateSnapshotRepository) GetActiveCities(ctx context.Context) ([]domain.City, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, state, is_active FROM cities WHERE is_active = TRUE ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("get active cities: %w", err)
	}
	defer rows.Close()

	var cities []domain.City
	for rows.Next() {
		var c domain.City
		if err := rows.Scan(&c.ID, &c.Name, &c.State, &c.IsActive); err != nil {
			return nil, err
		}
		cities = append(cities, c)
	}
	return cities, rows.Err()
}

// pgxRow is the interface satisfied by both pgx.Row and pgx.Rows (for scanSnapshot).
type pgxRow interface {
	Scan(dest ...any) error
}

func scanSnapshot(row pgxRow) (*domain.AIRateSnapshot, error) {
	var s domain.AIRateSnapshot
	var src string
	err := row.Scan(&s.ID, &s.CityID, &s.Gold, &s.Silver, &src, &s.IsStale, &s.GeneratedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// No snapshot exists for this city — callers interpret nil as "not found".
		return nil, nil
	}
	if err != nil {
		// Propagate real DB errors (network failure, type mismatch, etc.)
		// so callers can return 500 instead of silently treating missing data as 404.
		return nil, fmt.Errorf("scan snapshot: %w", err)
	}
	s.Source = domain.RateSource(src)
	return &s, nil
}
