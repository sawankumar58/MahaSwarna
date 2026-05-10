package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

type UserRepository struct{ db *pgxpool.Pool }

func NewUserRepository(db *pgxpool.Pool) *UserRepository { return &UserRepository{db: db} }

// UpsertUser inserts or does nothing. cityID written ONLY on fresh insert (xmax=0 guard).
func (r *UserRepository) UpsertUser(ctx context.Context, phone, cityID string) (*domain.User, bool, error) {
	id := uuid.New()
	var u domain.User
	var xmax string
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (id, phone, city_id, tier)
		VALUES ($1,$2,$3,'FREE')
		ON CONFLICT (phone) DO NOTHING
		RETURNING id, phone, city_id, tier, created_at, updated_at, xmax::text
	`, id, phone, cityID).Scan(&u.ID, &u.Phone, &u.CityID, &u.Tier, &u.CreatedAt, &u.UpdatedAt, &xmax)
	if err == pgx.ErrNoRows {
		u2, err2 := r.GetByPhone(ctx, phone)
		return u2, false, err2
	}
	if err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}
	return &u, xmax == "0", nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `SELECT id,phone,city_id,tier,created_at,updated_at,deleted_at,hard_deleted_at
		FROM users WHERE id=$1 AND deleted_at IS NULL`, id)
	return scanUser(row)
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `SELECT id,phone,city_id,tier,created_at,updated_at,deleted_at,hard_deleted_at
		FROM users WHERE phone=$1 AND deleted_at IS NULL`, phone)
	return scanUser(row)
}

func (r *UserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	return err
}

func (r *UserRepository) UpdateTier(ctx context.Context, id uuid.UUID, tier string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET tier=$1,updated_at=NOW() WHERE id=$2`, tier, id)
	return err
}

// UpdateCityID unconditionally overwrites city_id. Called only by RegisterUseCase;
// LoginUseCase uses UpsertUser which guards against overwriting on existing rows.
func (r *UserRepository) UpdateCityID(ctx context.Context, id uuid.UUID, cityID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET city_id=$1,updated_at=NOW() WHERE id=$2`, cityID, id)
	return err
}

func (r *UserRepository) PendingHardDeletes(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT id,phone,city_id,tier,created_at,updated_at,deleted_at,hard_deleted_at
		FROM users WHERE deleted_at IS NOT NULL AND deleted_at < NOW()-INTERVAL '30 days' AND hard_deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (r *UserRepository) MarkHardDeleted(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET hard_deleted_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *UserRepository) GetRecentUsersWithoutSubscription(ctx context.Context, window time.Duration) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT id,phone,city_id,tier,created_at,updated_at,deleted_at,hard_deleted_at
		FROM users
		WHERE created_at > NOW()-$1::interval AND deleted_at IS NULL
		  AND id NOT IN (SELECT user_id FROM subscriptions)`, window.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanUser(s scanner) (*domain.User, error) {
	var u domain.User
	var deletedAt, hardDeletedAt *time.Time
	if err := s.Scan(&u.ID, &u.Phone, &u.CityID, &u.Tier, &u.CreatedAt, &u.UpdatedAt, &deletedAt, &hardDeletedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	u.DeletedAt = deletedAt
	u.HardDeletedAt = hardDeletedAt
	return &u, nil
}
