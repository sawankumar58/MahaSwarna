package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReceiptLogRepository is append-only (REVOKE UPDATE,DELETE — migration 005).
type ReceiptLogRepository struct{ db *pgxpool.Pool }

func NewReceiptLogRepository(db *pgxpool.Pool) *ReceiptLogRepository { return &ReceiptLogRepository{db: db} }

func (r *ReceiptLogRepository) Insert(ctx context.Context,
	userID uuid.UUID, purchaseToken, productID, packageName, status string, raw any) error {
	b, _ := json.Marshal(raw)
	_, err := r.db.Exec(ctx, `INSERT INTO receipt_log
		(id,user_id,purchase_token,product_id,package_name,status,play_api_response,created_at)
		VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,NOW())
		ON CONFLICT(purchase_token) DO NOTHING`,
		userID, purchaseToken, productID, packageName, status, b)
	return err
}
