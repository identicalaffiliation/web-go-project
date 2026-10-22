package temporal

import (
	"context"
	"errors"

	"go.temporal.io/sdk/temporal"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
)

const (
	ReserveStockActivity = "inventory.ReserveStock"
	ReleaseStockActivity = "inventory.ReleaseStock"
)

const (
	ErrTypeInvalidReservation = "InvalidReservation"
	ErrTypeInsufficientStock  = "InsufficientStock"
	ErrTypeProductNotFound    = "ProductNotFound"
	ErrTypeOrderReleased      = "OrderReleased"
)

type stockService interface {
	ReserveStock(ctx context.Context, reservation *domain.StockReservation) error
	ReleaseStock(ctx context.Context, reservation *domain.StockReservation) error
}

type Activities struct {
	service stockService
}

func NewActivities(service stockService) *Activities {
	return &Activities{service: service}
}

func (a *Activities) ReserveStock(ctx context.Context, reservation *domain.StockReservation) error {
	return toTemporalError(a.service.ReserveStock(ctx, reservation))
}

func (a *Activities) ReleaseStock(ctx context.Context, reservation *domain.StockReservation) error {
	return toTemporalError(a.service.ReleaseStock(ctx, reservation))
}

func toTemporalError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidReservation):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeInvalidReservation, err)
	case errors.Is(err, domain.ErrInsufficientStock):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeInsufficientStock, err)
	case errors.Is(err, domain.ErrProductNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeProductNotFound, err)
	case errors.Is(err, domain.ErrOrderReleased):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeOrderReleased, err)
	default:
		return err
	}
}
