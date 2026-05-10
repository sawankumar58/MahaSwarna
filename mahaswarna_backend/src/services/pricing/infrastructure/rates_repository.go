package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/pricing/domain"
)

// RatesRepository manages the gold_rates table (manual_override rows only).
// Normal rates come exclusively from ai_rate_snapshots.
type RatesRepository struct {
	db *pgxpool.Pool
}

func NewRatesRepository(db *pgxpool.Pool) *RatesRepository {
	return &RatesRepository{db: db}
}

// GetLatestManual returns the most recent manual override rate for a city, or nil.
func (r *RatesRepository) GetLatestManual(ctx context.Context, cityID string) (*domain.GoldRate, error) {
	row := r.db.QueryRow(ctx,
		`SELECT city_id, gold, silver, source, is_stale, created_at
		 FROM gold_rates
		 WHERE city_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		cityID,
	)

	var gr domain.GoldRate
	var src string
	err := row.Scan(&gr.CityID, &gr.Gold, &gr.Silver, &src, &gr.IsStale, &gr.GeneratedAt)
	if err != nil {
		return nil, nil //nolint:nilerr — nil means no manual override exists
	}
	gr.Source = domain.RateSource(src)
	return &gr, nil
}

// InsertManual inserts a manual override rate. Used by the emergency override procedure
// documented in ARCHITECTURE.md when Gemini is down for >1 IST session.
func (r *RatesRepository) InsertManual(ctx context.Context, cityID string, gold, silver float64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO gold_rates (city_id, gold, silver, source)
		 VALUES ($1, $2, $3, 'manual_override')`,
		cityID, gold, silver,
	)
	if err != nil {
		return fmt.Errorf("insert manual rate %s: %w", cityID, err)
	}
	return nil
}
