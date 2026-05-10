package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/intelligence/domain"
)

// InvoiceRepository provides access to the invoices table.
// ADR-001: PDF bytes are never stored; only metadata is persisted.
type InvoiceRepository struct {
	pool *pgxpool.Pool
}

func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

// Insert persists invoice metadata (no PDF). Returns the stored invoice with its generated UUID.
func (r *InvoiceRepository) Insert(ctx context.Context, inv domain.Invoice) (*domain.Invoice, error) {
	itemsJSON, err := json.Marshal(inv.Items)
	if err != nil {
		return nil, fmt.Errorf("marshal invoice items: %w", err)
	}

	const q = `
		INSERT INTO invoices (
			shop_id, user_id, customer_name, customer_phone,
			items, payment_mode, notes,
			gold_rate_override, silver_rate_override,
			rate_source, pdf_size_bytes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, generated_at
	`
	var id uuid.UUID
	var generatedAt time.Time
	err = r.pool.QueryRow(ctx, q,
		inv.ShopID, inv.UserID, inv.CustomerName, inv.CustomerPhone,
		itemsJSON, string(inv.PaymentMode), inv.Notes,
		inv.GoldRateOverride, inv.SilverRateOverride,
		string(inv.RateSource), inv.PDFSizeBytes,
	).Scan(&id, &generatedAt)
	if err != nil {
		return nil, fmt.Errorf("invoice insert: %w", err)
	}

	inv.ID = id
	inv.GeneratedAt = generatedAt
	return &inv, nil
}

// ListByShop returns invoice metadata for a shop, newest first, with cursor-based pagination.
func (r *InvoiceRepository) ListByShop(ctx context.Context, shopID uuid.UUID, limit int, before *time.Time) ([]domain.Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows pgx.Rows
	var err error

	if before != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, shop_id, user_id, customer_name, customer_phone,
			       items, payment_mode, notes,
			       gold_rate_override, silver_rate_override,
			       rate_source, pdf_size_bytes, generated_at
			FROM invoices
			WHERE shop_id = $1 AND generated_at < $2
			ORDER BY generated_at DESC
			LIMIT $3
		`, shopID, before, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, shop_id, user_id, customer_name, customer_phone,
			       items, payment_mode, notes,
			       gold_rate_override, silver_rate_override,
			       rate_source, pdf_size_bytes, generated_at
			FROM invoices
			WHERE shop_id = $1
			ORDER BY generated_at DESC
			LIMIT $2
		`, shopID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("invoice list: %w", err)
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Invoice, error) {
		return scanInvoice(row)
	})
}

// DeleteByShopID removes all invoices for a shop. Called during GDPR erasure.
func (r *InvoiceRepository) DeleteByShopID(ctx context.Context, shopID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM invoices WHERE shop_id = $1`, shopID)
	return err
}

func scanInvoice(row pgx.CollectableRow) (domain.Invoice, error) {
	var inv domain.Invoice
	var itemsJSON []byte
	var paymentMode, rateSource string
	err := row.Scan(
		&inv.ID, &inv.ShopID, &inv.UserID,
		&inv.CustomerName, &inv.CustomerPhone,
		&itemsJSON, &paymentMode, &inv.Notes,
		&inv.GoldRateOverride, &inv.SilverRateOverride,
		&rateSource, &inv.PDFSizeBytes, &inv.GeneratedAt,
	)
	if err != nil {
		return domain.Invoice{}, err
	}
	inv.PaymentMode = domain.PaymentMode(paymentMode)
	inv.RateSource = domain.RateSource(rateSource)
	if err := json.Unmarshal(itemsJSON, &inv.Items); err != nil {
		return domain.Invoice{}, fmt.Errorf("unmarshal invoice items: %w", err)
	}
	return inv, nil
}
