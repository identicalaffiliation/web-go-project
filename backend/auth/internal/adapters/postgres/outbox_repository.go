package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type OutboxStore struct {
	pool *pgxpool.Pool
}

func NewOutboxStore(pool *pgxpool.Pool) *OutboxStore {
	return &OutboxStore{pool: pool}
}

var _ ports.OutboxStore = (*OutboxStore)(nil)

func (s *OutboxStore) Begin(ctx context.Context) (ports.OutboxBatch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin outbox tx: %w", err)
	}
	return &outboxBatch{tx: tx}, nil
}

type outboxBatch struct {
	tx pgx.Tx
}

// Fetch берёт неотправленные события и блокирует их до конца транзакции.
// SKIP LOCKED: строки, уже занятые другой репликой, пропускаются, а не ждутся,
// поэтому две реплики никогда не публикуют одну запись одновременно.
func (b *outboxBatch) Fetch(ctx context.Context, limit, maxAttempts int) ([]domain.OutboxEvent, error) {
	const query = `
		SELECT id, topic, key, payload, created_at, attempts
		FROM outbox
		WHERE sent_at IS NULL AND attempts < $2
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := b.tx.Query(ctx, query, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("select outbox events: %w", err)
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(&e.ID, &e.Topic, &e.Key, &e.Payload, &e.CreatedAt, &e.Attempts); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func (b *outboxBatch) MarkSent(ctx context.Context, ids []uuid.UUID) error {
	if _, err := b.tx.Exec(ctx, `UPDATE outbox SET sent_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("mark outbox events sent: %w", err)
	}
	return nil
}

func (b *outboxBatch) MarkFailed(ctx context.Context, ids []uuid.UUID) error {
	if _, err := b.tx.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1 WHERE id = ANY($1)`, ids); err != nil {
		return fmt.Errorf("increment outbox attempts: %w", err)
	}
	return nil
}

func (b *outboxBatch) Commit(ctx context.Context) error {
	if err := b.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit outbox tx: %w", err)
	}
	return nil
}

func (b *outboxBatch) Rollback(ctx context.Context) error {
	if err := b.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback outbox tx: %w", err)
	}
	return nil
}

func insertOutboxEvent(ctx context.Context, q execer, e domain.OutboxEvent) error {
	const query = `
		INSERT INTO outbox (id, topic, key, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	if _, err := q.Exec(ctx, query, e.ID, e.Topic, e.Key, e.Payload, e.CreatedAt); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
