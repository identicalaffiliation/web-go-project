//go:build unit

package temporal

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	sdktemporal "go.temporal.io/sdk/temporal"

	"github.com/identicalaffiliation/web-go-project/backend/orders/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

func TestActivities_UpdateOrderStatus(t *testing.T) {
	ctx := context.Background()
	orderID := uuid.NewV7()

	t.Run("updates status", func(t *testing.T) {
		repo := mocks.NewMockStatusUpdater(t)
		repo.EXPECT().UpdateStatusByOrderID(ctx, orderID, domain.Paid).Return(nil).Once()

		require.NoError(t, NewActivities(repo).UpdateOrderStatus(ctx, orderID, domain.Paid))
	})

	t.Run("not found is non retryable", func(t *testing.T) {
		repo := mocks.NewMockStatusUpdater(t)
		repo.EXPECT().UpdateStatusByOrderID(ctx, orderID, domain.Failed).Return(postgres.ErrNotFound).Once()

		err := NewActivities(repo).UpdateOrderStatus(ctx, orderID, domain.Failed)

		appErr, ok := errors.AsType[*sdktemporal.ApplicationError](err)
		require.True(t, ok)
		require.True(t, appErr.NonRetryable())
		require.Equal(t, ErrTypeOrderNotFound, appErr.Type())
	})

	t.Run("db error is retryable", func(t *testing.T) {
		repo := mocks.NewMockStatusUpdater(t)
		dbErr := errors.New("db down")
		repo.EXPECT().UpdateStatusByOrderID(ctx, orderID, domain.Paid).Return(dbErr).Once()

		err := NewActivities(repo).UpdateOrderStatus(ctx, orderID, domain.Paid)
		require.ErrorIs(t, err, dbErr)

		_, isAppErr := errors.AsType[*sdktemporal.ApplicationError](err)
		require.False(t, isAppErr)
	})
}
