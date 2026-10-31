package likes_outbox

import (
	"context"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	poolRO *pgxpool.Pool
	pool   *pgxpool.Pool
	getter *trm.CtxGetter
}

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		poolRO: replica,
		pool:   master,
		getter: trm.DefaultCtxGetter,
	}
}

func (r *Repository) Insert(ctx context.Context, event *domain.Event) error {
	const query = `INSERT INTO outbox_likes (id, key, payload) VALUES ($1, $2, $3)`

	conn := r.getter.DefaultTrOrDB(ctx, r.pool)
	_, err := conn.Exec(ctx, query, event.EventID, event.Key, event.Payload)
	if err != nil {
		return fmt.Errorf("error Insert: %w", err)
	}

	return nil
}

// GetEvents - пачка неотправленных событий. Читает master: с реплики (лаг) приходят уже
// отправленные события. Внутри транзакции строки блокируются до коммита, а занятые другой
// репликой воркера пропускаются - одно событие забирает только одна реплика.
func (r *Repository) GetEvents(ctx context.Context, limit int64) ([]*domain.Event, error) {
	const query = `
		SELECT id, key, payload, created_at, sent_at FROM outbox_likes
		WHERE sent_at IS NULL ORDER BY created_at LIMIT $1
		FOR UPDATE SKIP LOCKED`

	conn := r.getter.DefaultTrOrDB(ctx, r.pool)
	rows, err := conn.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get unsent rows: %w", err)
	}

	t, err := pgx.CollectRows(rows, pgx.RowToStructByName[eventModel])
	if err != nil {
		return nil, err
	}

	e := make([]*domain.Event, len(t))
	for i, v := range t {
		e[i] = v.toDomain()
	}

	return e, nil
}

func (r *Repository) SentEvent(ctx context.Context, ids []uuid.UUID) error {
	const query = `UPDATE outbox_likes SET sent_at = now() WHERE id = ANY($1)`

	if _, err := r.getter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, ids); err != nil {
		return fmt.Errorf("update rows: %w", err)
	}

	return nil
}
