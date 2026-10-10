//go:build integration

package integration

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getPool(ctx context.Context, dsn string) *pgxpool.Pool {
	pool, _ := pgxpool.New(ctx, dsn)
	return pool
}
