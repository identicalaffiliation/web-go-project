package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

type Repository interface {
	CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error)
	GetOrdersByUserID(ctx context.Context, id uuid.UUID) ([]*domain.Order, error)
	UpdateStatusByOrderID(ctx context.Context, id uuid.UUID, newStatus domain.Status) error
}
