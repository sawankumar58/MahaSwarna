package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

const RefreshTTL = 30 * 24 * time.Hour

type SessionRepository struct{ db *pgxpool.Pool }

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository { return &SessionRepository{db: db} }

func (r *SessionRepository) Create(ctx context.Context, s domain.Session) error {
	_, err := r.db.Exec(ctx, `INSERT INTO sessions(jti,user_id,revoked,created_at,expires_at)
		VALUES($1,$2,false,NOW(),$3)`, s.JTI, s.UserID, s.ExpiresAt)
	return err
}

func (r *SessionRepository) IsRevoked(ctx context.Context, jti uuid.UUID) (bool, error) {
	var revoked bool
	err := r.db.QueryRow(ctx, `SELECT revoked FROM sessions WHERE jti=$1`, jti).Scan(&revoked)
	if err != nil {
		return true, fmt.Errorf("check revoked: %w", err)
	}
	return revoked, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, jti uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked=true WHERE jti=$1`, jti)
	return err
}

func (r *SessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1 AND revoked=false`, userID)
	return err
}

func (r *SessionRepository) GetByJTI(ctx context.Context, jti uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx, `SELECT jti,user_id,revoked,created_at,expires_at
		FROM sessions WHERE jti=$1 AND revoked=false AND expires_at>NOW()`, jti).
		Scan(&s.JTI, &s.UserID, &s.Revoked, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
