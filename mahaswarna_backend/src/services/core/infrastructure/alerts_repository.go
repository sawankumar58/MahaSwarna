package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

type AlertsRepository struct{ db *pgxpool.Pool }

func NewAlertsRepository(db *pgxpool.Pool) *AlertsRepository { return &AlertsRepository{db: db} }

func (r *AlertsRepository) Create(ctx context.Context, a domain.Alert) (*domain.Alert, error) {
	a.ID = uuid.New(); a.CreatedAt = time.Now()
	_, err := r.db.Exec(ctx, `INSERT INTO alerts(id,user_id,city_id,metal,threshold,direction,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, a.ID,a.UserID,a.CityID,a.Metal,a.Threshold,a.Direction,a.CreatedAt)
	if err != nil { return nil, err }
	return &a, nil
}

func (r *AlertsRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Alert, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,city_id,metal,threshold,direction,created_at,delivered_at
		FROM alerts WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanAlerts(rows)
}

func (r *AlertsRepository) ListPendingByCityMetal(ctx context.Context, cityID, metal string) ([]domain.Alert, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,city_id,metal,threshold,direction,created_at,delivered_at
		FROM alerts WHERE city_id=$1 AND metal=$2 AND delivered_at IS NULL`, cityID, metal)
	if err != nil { return nil, err }
	defer rows.Close()
	return scanAlerts(rows)
}

func (r *AlertsRepository) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE alerts SET delivered_at=NOW() WHERE id=$1`, id)
	return err
}

// CityMetalPair is a distinct (city_id, metal) combination that has at least one
// pending (undelivered) alert. Used by AlertThresholdJob to drive evaluation without
// a hardcoded city list.
type CityMetalPair struct {
	CityID string
	Metal  string
}

// ListActiveCityMetalPairs returns every distinct (city_id, metal) pair that has at
// least one alert with delivered_at IS NULL. The job calls this once per minute and
// then evaluates each pair — ensuring all 61 supported cities are covered.
func (r *AlertsRepository) ListActiveCityMetalPairs(ctx context.Context) ([]CityMetalPair, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT city_id, metal FROM alerts WHERE delivered_at IS NULL ORDER BY city_id, metal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []CityMetalPair
	for rows.Next() {
		var p CityMetalPair
		if err := rows.Scan(&p.CityID, &p.Metal); err != nil {
			return nil, err
		}
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

func (r *AlertsRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM alerts WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil { return err }
	if tag.RowsAffected() == 0 { return fmt.Errorf("alert not found") }
	return nil
}

type rowScanner interface { Next() bool; Scan(...any) error; Err() error }

func scanAlerts(rows rowScanner) ([]domain.Alert, error) {
	var alerts []domain.Alert
	for rows.Next() {
		var a domain.Alert
		if err := rows.Scan(&a.ID,&a.UserID,&a.CityID,&a.Metal,&a.Threshold,&a.Direction,&a.CreatedAt,&a.DeliveredAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}
