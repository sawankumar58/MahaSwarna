package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

// ConsentLogRepository is insert-only (REVOKE UPDATE,DELETE enforced at DB layer — migration 003).
type ConsentLogRepository struct{ db *pgxpool.Pool }

func NewConsentLogRepository(db *pgxpool.Pool) *ConsentLogRepository { return &ConsentLogRepository{db: db} }

func (r *ConsentLogRepository) Upsert(ctx context.Context, c domain.ConsentLog) (bool, error) {
	tag, err := r.db.Exec(ctx, `INSERT INTO consent_log(id,user_id,consent_type,version,consented_at)
		VALUES(gen_random_uuid(),$1,$2,$3,NOW())
		ON CONFLICT(user_id,consent_type,version) DO NOTHING`,
		c.UserID, c.ConsentType, c.Version)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ConsentLogRepository) GetLatest(ctx context.Context, userID, consentType string) (*domain.ConsentLog, error) {
	var cl domain.ConsentLog
	err := r.db.QueryRow(ctx, `SELECT id,user_id,consent_type,version,consented_at
		FROM consent_log WHERE user_id=$1 AND consent_type=$2
		ORDER BY consented_at DESC LIMIT 1`, userID, consentType).
		Scan(&cl.ID, &cl.UserID, &cl.ConsentType, &cl.Version, &cl.ConsentedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &cl, err
}
