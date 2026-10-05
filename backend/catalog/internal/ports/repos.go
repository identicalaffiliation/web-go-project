package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type CatalogRepository interface {
	Insert(ctx context.Context, product *domain.Product) (*domain.Product, error)
	GetItemByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetItemsByCursorPagination(ctx context.Context, cursor *string, limit int64) ([]*domain.Product, error)
}

type OutboxRepository interface {
	Insert(ctx context.Context, event *domain.Event) error
	SentEvent(ctx context.Context, ids []uuid.UUID) error
	GetEvents(ctx context.Context, limit int64) ([]*domain.Event, error)
}
