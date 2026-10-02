package outbox

import (
	"context"
	"fmt"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	poolRO *pgxpool.Pool
	getter *trm.CtxGetter
}

func NewRepository(pool1, pool2 *pgxpool.Pool) *Repository {
	return &Repository{pool: pool1, poolRO: pool2, getter: trm.DefaultCtxGetter}
}

func (r *Repository) Insert(ctx context.Context, event *domain.Event) error {
	const query = `INSERT INTO outbox (id, key, payload) VALUES ($1, $2, $3)`

	conn := r.getter.DefaultTrOrDB(ctx, r.pool)
	_, err := conn.Exec(ctx, query, event.EventID, event.Key, event.Payload)
	if err != nil {
		return fmt.Errorf("error OutboxInsert: %w", err)
	}

	return nil
}
