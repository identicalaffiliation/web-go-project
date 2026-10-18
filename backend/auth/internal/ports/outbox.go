package ports

//go:generate go tool mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type OutboxStore interface {
	Begin(ctx context.Context) (OutboxBatch, error)
}

type OutboxBatch interface {
	Fetch(ctx context.Context, limit, maxAttempts int) ([]domain.OutboxEvent, error)
	MarkSent(ctx context.Context, ids []uuid.UUID) error
	MarkFailed(ctx context.Context, ids []uuid.UUID) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type EventPublisher interface {
	Publish(ctx context.Context, events []domain.OutboxEvent) error
}

type PartialPublishError struct {
	Rejected    map[uuid.UUID]error
	Undelivered map[uuid.UUID]error
}

func (e *PartialPublishError) Error() string {
	return fmt.Sprintf("partial publish: %d rejected, %d undelivered", len(e.Rejected), len(e.Undelivered))
}
