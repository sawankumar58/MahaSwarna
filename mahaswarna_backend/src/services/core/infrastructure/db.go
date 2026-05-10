package infrastructure

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/infrastructure/postgres"
)

func NewDB(ctx context.Context) (*pgxpool.Pool, error) {
	return postgres.NewPool(ctx, "core")
}
