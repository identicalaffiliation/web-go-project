package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"go.uber.org/zap"
)

type Logger interface {
	With(fields ...zap.Field) Logger
	Debug(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	WithError(err error) Logger
	Sync() error
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type PriceProvider interface {
	GetPrice(ctx context.Context, productID uuid.UUID) (int64, error)
}

type WorkflowStarter interface {
	StartOrderSaga(ctx context.Context, order *domain.Order) error
}
