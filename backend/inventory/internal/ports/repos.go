package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
)

type StockRepository interface {
	CreateStock(ctx context.Context, stock *domain.Stock) error
	ReserveStock(ctx context.Context, quantity int16, productID uuid.UUID) (int16, error)
	ReleaseStock(ctx context.Context, quantity int16, productID uuid.UUID) (int16, error)
	LockOrder(ctx context.Context, orderID uuid.UUID) error
	CheckIdempotencyKey(ctx context.Context, orderID uuid.UUID, op domain.Operation) (bool, error)
	SaveIdempotencyKey(ctx context.Context, orderID uuid.UUID, op domain.Operation) error
}
