package temporal

import (
	"context"
	"errors"

	"go.temporal.io/sdk/temporal"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

const (
	ChargeActivity = "payments.Charge"
	RefundActivity = "payments.Refund"
)

const (
	ErrTypeInvalidPayment    = "InvalidPayment"
	ErrTypeInsufficientFunds = "InsufficientFunds"
	ErrTypeAccountNotFound   = "AccountNotFound"
	ErrTypeOrderRefunded     = "OrderRefunded"
)

type paymentService interface {
	Charge(ctx context.Context, payment *domain.Payment) error
	Refund(ctx context.Context, payment *domain.Payment) error
}

type Activities struct {
	service paymentService
}

func NewActivities(service paymentService) *Activities {
	return &Activities{service: service}
}

func (a *Activities) Charge(ctx context.Context, payment *domain.Payment) error {
	return toTemporalError(a.service.Charge(ctx, payment))
}

func (a *Activities) Refund(ctx context.Context, payment *domain.Payment) error {
	return toTemporalError(a.service.Refund(ctx, payment))
}

// toTemporalError - бизнес-ошибки не ретраятся, сага сразу уходит в компенсацию.
// Остальные (бд недоступна и т.п.) ретраятся по политике workflow.
func toTemporalError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrInvalidPayment):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeInvalidPayment, err)
	case errors.Is(err, domain.ErrInsufficientFunds):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeInsufficientFunds, err)
	case errors.Is(err, domain.ErrAccountNotFound):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeAccountNotFound, err)
	case errors.Is(err, domain.ErrOrderRefunded):
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeOrderRefunded, err)
	default:
		return err
	}
}
