package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type CatalogRepository interface {
	Insert(ctx context.Context, product *domain.Product) (*domain.Product, error)
	GetItemByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	GetItemsByCursorPagination(ctx context.Context, cursor *string, limit int64) ([]*domain.Product, error)
	UpdateImageKey(ctx context.Context, key string, productID uuid.UUID) (*domain.Product, error)
}

type OutboxRepository interface {
	Insert(ctx context.Context, event *domain.Event) error
	SentEvent(ctx context.Context, ids []uuid.UUID) error
	GetEvents(ctx context.Context, limit int64) ([]*domain.Event, error)
}

type FeedbackRepository interface {
	CreateFeedback(ctx context.Context, f *domain.Feedback) (*domain.Feedback, error)
	GetMyCreatedFeedbacks(ctx context.Context, userId uuid.UUID) ([]*domain.Feedback, error)
	DeleteFeedback(ctx context.Context, id uuid.UUID) error
	GetProductFeedbacks(ctx context.Context, productID uuid.UUID, cursor *uuid.UUID, limit int64) ([]*domain.Feedback, error)
}
