package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/intelligence/domain"
)

// ShopRepository provides access to the shops table.
type ShopRepository struct {
	pool *pgxpool.Pool
}

func NewShopRepository(pool *pgxpool.Pool) *ShopRepository {
	return &ShopRepository{pool: pool}
}

// Insert creates a new shop. Returns ErrShopAlreadyExists if user_id already has one
// (unique constraint violation on idx_shops_user_id).
func (r *ShopRepository) Insert(ctx context.Context, s domain.Shop) (*domain.Shop, error) {
	const q = `
		INSERT INTO shops (user_id, name, address, gst_number, phone)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, address, gst_number, phone,
		          banner_url, banner_object_key, created_at, updated_at
	`
	rows, err := r.pool.Query(ctx, q,
		s.UserID, s.Name, s.Address, s.GSTNumber, s.Phone,
	)
	if err != nil {
		return nil, fmt.Errorf("shop insert: %w", err)
	}
	defer rows.Close()

	shop, err := pgx.CollectOneRow(rows, scanShop)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 = unique_violation — user already has a shop.
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrShopAlreadyExists{}
		}
		return nil, fmt.Errorf("shop insert scan: %w", err)
	}
	return &shop, nil
}

// GetByUserID returns the shop owned by a user, or nil if none exists.
func (r *ShopRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Shop, error) {
	const q = `
		SELECT id, user_id, name, address, gst_number, phone,
		       banner_url, banner_object_key, created_at, updated_at
		FROM shops WHERE user_id = $1
	`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("shop get by user: %w", err)
	}
	defer rows.Close()

	shop, err := pgx.CollectOneRow(rows, scanShop)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shop get by user scan: %w", err)
	}
	return &shop, nil
}

// GetByID returns a shop by primary key.
func (r *ShopRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Shop, error) {
	const q = `
		SELECT id, user_id, name, address, gst_number, phone,
		       banner_url, banner_object_key, created_at, updated_at
		FROM shops WHERE id = $1
	`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("shop get by id: %w", err)
	}
	defer rows.Close()

	shop, err := pgx.CollectOneRow(rows, scanShop)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shop get by id scan: %w", err)
	}
	return &shop, nil
}

// UpdateBanner sets banner_url and banner_object_key atomically.
// Used by ConfirmBannerUploadUseCase after moderation passes.
func (r *ShopRepository) UpdateBanner(ctx context.Context, shopID uuid.UUID, bannerURL, objectKey string) error {
	const q = `
		UPDATE shops
		SET banner_url = $1, banner_object_key = $2
		WHERE id = $3
	`
	tag, err := r.pool.Exec(ctx, q, bannerURL, objectKey, shopID)
	if err != nil {
		return fmt.Errorf("shop update banner: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("shop not found: %s", shopID)
	}
	return nil
}

// DeleteByUserID removes all shops belonging to a user. Called by the
// account_deleted event listener during GDPR erasure.
func (r *ShopRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM shops WHERE user_id = $1`, userID)
	return err
}

// ListAll returns all shop records. Used only by RepopulateSubscriptionProjection
// during a pg NOTIFY reconnect to re-seed the Redis read model for every active shop owner.
func (r *ShopRepository) ListAll(ctx context.Context) ([]domain.Shop, error) {
	const q = `
		SELECT id, user_id, name, address, gst_number, phone,
		       banner_url, banner_object_key, created_at, updated_at
		FROM shops
		ORDER BY created_at
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("shop list all: %w", err)
	}
	defer rows.Close()

	shops, err := pgx.CollectRows(rows, scanShop)
	if err != nil {
		return nil, fmt.Errorf("shop list all scan: %w", err)
	}
	return shops, nil
}

func scanShop(row pgx.CollectableRow) (domain.Shop, error) {
	var s domain.Shop
	return s, row.Scan(
		&s.ID, &s.UserID, &s.Name, &s.Address, &s.GSTNumber, &s.Phone,
		&s.BannerURL, &s.BannerObjectKey, &s.CreatedAt, &s.UpdatedAt,
	)
}
