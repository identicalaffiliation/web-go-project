//go:build unit

package temporal

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	sdktemporal "go.temporal.io/sdk/temporal"

	"github.com/identicalaffiliation/web-go-project/backend/payments/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

func TestToTemporalError(t *testing.T) {
	nonRetryable := map[string]struct {
		err     error
		errType string
	}{
		"invalid payment":    {domain.ErrInvalidPayment, ErrTypeInvalidPayment},
		"insufficient funds": {domain.ErrInsufficientFunds, ErrTypeInsufficientFunds},
		"account not found":  {domain.ErrAccountNotFound, ErrTypeAccountNotFound},
		"order refunded":     {domain.ErrOrderRefunded, ErrTypeOrderRefunded},
	}

	for name, tc := range nonRetryable {
		t.Run(name, func(t *testing.T) {
			appErr, ok := errors.AsType[*sdktemporal.ApplicationError](toTemporalError(tc.err))
			require.True(t, ok)
			require.True(t, appErr.NonRetryable())
			require.Equal(t, tc.errType, appErr.Type())
		})
	}

	t.Run("nil", func(t *testing.T) {
		require.NoError(t, toTemporalError(nil))
	})

	t.Run("db error is retryable", func(t *testing.T) {
		dbErr := errors.New("db down")

		err := toTemporalError(dbErr)
		require.ErrorIs(t, err, dbErr)

		_, isAppErr := errors.AsType[*sdktemporal.ApplicationError](err)
		require.False(t, isAppErr)
	})
}

func TestActivities(t *testing.T) {
	ctx := context.Background()
	payment := &domain.Payment{OrderID: uuid.NewV7(), UserID: uuid.NewV7(), Amount: 100}

	t.Run("charge", func(t *testing.T) {
		svc := mocks.NewMockPaymentService(t)
		svc.EXPECT().Charge(ctx, payment).Return(nil).Once()

		require.NoError(t, NewActivities(svc).Charge(ctx, payment))
	})

	t.Run("charge insufficient funds is non retryable", func(t *testing.T) {
		svc := mocks.NewMockPaymentService(t)
		svc.EXPECT().Charge(ctx, payment).Return(domain.ErrInsufficientFunds).Once()

		appErr, ok := errors.AsType[*sdktemporal.ApplicationError](NewActivities(svc).Charge(ctx, payment))
		require.True(t, ok)
		require.True(t, appErr.NonRetryable())
	})

	t.Run("refund", func(t *testing.T) {
		svc := mocks.NewMockPaymentService(t)
		svc.EXPECT().Refund(ctx, payment).Return(nil).Once()

		require.NoError(t, NewActivities(svc).Refund(ctx, payment))
	})
}
