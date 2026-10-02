package ports

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type CatalogRepository interface {
	Insert(ctx context.Context, product *domain.Product) (*domain.Product, error)
}

type OutboxRepository interface {
	Insert(ctx context.Context, event *domain.Event) error
}
