package infrastructure

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
)

type DeviceTokenRepository struct{ db *pgxpool.Pool }

func NewDeviceTokenRepository(db *pgxpool.Pool) *DeviceTokenRepository { return &DeviceTokenRepository{db: db} }

func (r *DeviceTokenRepository) Upsert(ctx context.Context, dt domain.DeviceToken) error {
	_, err := r.db.Exec(ctx, `INSERT INTO device_tokens(id,user_id,device_id,token,platform,created_at,updated_at)
		VALUES(gen_random_uuid(),$1,$2,$3,$4,NOW(),NOW())
		ON CONFLICT(user_id,device_id) DO UPDATE SET token=EXCLUDED.token,platform=EXCLUDED.platform,updated_at=NOW()`,
		dt.UserID, dt.DeviceID, dt.Token, dt.Platform)
	return err
}

func (r *DeviceTokenRepository) GetTokensForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT token FROM device_tokens WHERE user_id=$1`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil { return nil, err }
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// Delete removes a specific FCM token by its token string, scoped to the owning user.
// Scoping by userID prevents one user from deregistering another user's token.
// Returns nil if the token did not exist (idempotent).
func (r *DeviceTokenRepository) Delete(ctx context.Context, userID uuid.UUID, token string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM device_tokens WHERE user_id=$1 AND token=$2`,
		userID, token)
	return err
}
