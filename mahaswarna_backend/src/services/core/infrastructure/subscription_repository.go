package infrastructure

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

type SubscriptionRepository struct{ db *pgxpool.Pool }

func NewSubscriptionRepository(db *pgxpool.Pool) *SubscriptionRepository { return &SubscriptionRepository{db: db} }

func (r *SubscriptionRepository) Upsert(ctx context.Context, s domain.Subscription) error {
	_, err := r.db.Exec(ctx, `INSERT INTO subscriptions
		(id,user_id,tier,purchase_token,product_id,package_name,status,activated_at,expires_at,created_at,updated_at)
		VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,'ACTIVE',NOW(),$6,NOW(),NOW())
		ON CONFLICT(purchase_token) DO NOTHING`,
		s.UserID, s.Tier, s.PurchaseToken, s.ProductID, s.PackageName, s.ExpiresAt)
	return err
}

func (r *SubscriptionRepository) GetActiveByUserID(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	err := r.db.QueryRow(ctx, `SELECT id,user_id,tier,purchase_token,product_id,package_name,
		status,activated_at,expires_at,created_at,updated_at
		FROM subscriptions WHERE user_id=$1 AND status='ACTIVE'
		AND (expires_at IS NULL OR expires_at>NOW())
		ORDER BY activated_at DESC LIMIT 1`, userID).
		Scan(&s.ID,&s.UserID,&s.Tier,&s.PurchaseToken,&s.ProductID,&s.PackageName,
			&s.Status,&s.ActivatedAt,&s.ExpiresAt,&s.CreatedAt,&s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

func (r *SubscriptionRepository) ListAllActive(ctx context.Context) ([]domain.Subscription, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,tier,purchase_token,product_id,package_name,
		status,activated_at,expires_at,created_at,updated_at
		FROM subscriptions WHERE status='ACTIVE' AND (expires_at IS NULL OR expires_at>NOW())`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(&s.ID,&s.UserID,&s.Tier,&s.PurchaseToken,&s.ProductID,&s.PackageName,
			&s.Status,&s.ActivatedAt,&s.ExpiresAt,&s.CreatedAt,&s.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *SubscriptionRepository) ExpireOverdue(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `UPDATE subscriptions SET status='EXPIRED',updated_at=NOW()
		WHERE status='ACTIVE' AND expires_at IS NOT NULL AND expires_at<NOW()`)
	return tag.RowsAffected(), err
}

func (r *SubscriptionRepository) InsertFree(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `INSERT INTO subscriptions
		(id,user_id,tier,purchase_token,product_id,package_name,status,activated_at,expires_at,created_at,updated_at)
		VALUES(gen_random_uuid(),$1,'FREE','','free','com.mahaswarna','ACTIVE',NOW(),NULL,NOW(),NOW())
		ON CONFLICT DO NOTHING`, userID)
	return err
}

var _ = time.Now // keep time import
