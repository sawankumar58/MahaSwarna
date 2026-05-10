package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/shared"
)

// AuditLogRepository is append-only (REVOKE UPDATE,DELETE — migration 010).
type AuditLogRepository struct{ db *pgxpool.Pool }

func NewAuditLogRepository(db *pgxpool.Pool) *AuditLogRepository { return &AuditLogRepository{db: db} }

func (r *AuditLogRepository) Append(ctx context.Context, entry shared.AuditEntry) error {
	meta, _ := json.Marshal(entry.Metadata)
	_, err := r.db.Exec(ctx, `INSERT INTO audit_log(id,actor,action,entity,entity_id,metadata,created_at)
		VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,NOW())`,
		entry.Actor, entry.Action, entry.Entity, entry.EntityID, meta)
	return err
}
