//go:build unit

package workflow

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
)

type testDeps struct {
	repo    *mocks.MockStockRepository
	logger  *mocks.MockLogger
	manager *mocks.MockTxManager
}

func setup(t *testing.T) (*Service, testDeps) {
	t.Helper()

	deps := testDeps{
		repo:    mocks.NewMockStockRepository(t),
		logger:  mocks.NewMockLogger(t),
		manager: mocks.NewMockTxManager(t),
	}

	deps.logger.EXPECT().With(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().WithError(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().Error(mock.Anything, mock.Anything).Maybe()
	deps.manager.EXPECT().Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		}).Maybe()

	return NewService(deps.repo, deps.logger, deps.manager), deps
}

func TestService_ReserveStock(t *testing.T) {
	ctx := context.Background()
	p1 := uuid.New()
	order := uuid.New()
	reservation := &domain.StockReservation{OrderID: order, ProductID: p1, Quantity: 3}
	dbErr := errors.New("db down")

	t.Run("reserves stock and saves idempotency key", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(false, nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(false, nil).Once()
		deps.repo.EXPECT().ReserveStock(mock.Anything, int16(3), p1).Return(7, nil).Once()
		deps.repo.EXPECT().SaveIdempotencyKey(mock.Anything, order, domain.Reserve).Return(nil).Once()

		require.NoError(t, s.ReserveStock(ctx, reservation))
	})

	t.Run("already reserved is no-op", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(true, nil).Once()

		require.NoError(t, s.ReserveStock(ctx, reservation))
	})

	t.Run("reserve after release is rejected", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(false, nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(true, nil).Once()

		require.ErrorIs(t, s.ReserveStock(ctx, reservation), domain.ErrOrderReleased)
	})

	t.Run("insufficient stock does not save key", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, mock.Anything).Return(false, nil).Twice()
		deps.repo.EXPECT().ReserveStock(mock.Anything, int16(3), p1).Return(0, domain.ErrInsufficientStock).Once()

		require.ErrorIs(t, s.ReserveStock(ctx, reservation), domain.ErrInsufficientStock)
	})

	t.Run("lock error", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(dbErr).Once()

		require.ErrorIs(t, s.ReserveStock(ctx, reservation), dbErr)
	})

	t.Run("check idempotency key error", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(false, dbErr).Once()

		require.ErrorIs(t, s.ReserveStock(ctx, reservation), dbErr)
	})

	t.Run("save idempotency key error", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, mock.Anything).Return(false, nil).Twice()
		deps.repo.EXPECT().ReserveStock(mock.Anything, int16(3), p1).Return(7, nil).Once()
		deps.repo.EXPECT().SaveIdempotencyKey(mock.Anything, order, domain.Reserve).Return(dbErr).Once()

		require.ErrorIs(t, s.ReserveStock(ctx, reservation), dbErr)
	})

	t.Run("invalid input does not touch repository", func(t *testing.T) {
		s, _ := setup(t)
		for _, r := range []*domain.StockReservation{
			nil,
			{ProductID: p1, Quantity: 1},
			{OrderID: order, Quantity: 1},
			{OrderID: order, ProductID: p1},
			{OrderID: order, ProductID: p1, Quantity: -1},
		} {
			require.ErrorIs(t, s.ReserveStock(ctx, r), domain.ErrInvalidReservation)
		}
	})
}

func TestService_ReleaseStock(t *testing.T) {
	ctx := context.Background()
	p1 := uuid.New()
	order := uuid.New()
	reservation := &domain.StockReservation{OrderID: order, ProductID: p1, Quantity: 3}
	dbErr := errors.New("db down")

	t.Run("returns reserved stock and saves key", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(false, nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(true, nil).Once()
		deps.repo.EXPECT().ReleaseStock(mock.Anything, int16(3), p1).Return(10, nil).Once()
		deps.repo.EXPECT().SaveIdempotencyKey(mock.Anything, order, domain.Release).Return(nil).Once()

		require.NoError(t, s.ReleaseStock(ctx, reservation))
	})

	t.Run("without reserve only saves key", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(false, nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(false, nil).Once()
		deps.repo.EXPECT().SaveIdempotencyKey(mock.Anything, order, domain.Release).Return(nil).Once()

		require.NoError(t, s.ReleaseStock(ctx, reservation))
	})

	t.Run("already released is no-op", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(true, nil).Once()

		require.NoError(t, s.ReleaseStock(ctx, reservation))
	})

	t.Run("release error", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Release).Return(false, nil).Once()
		deps.repo.EXPECT().CheckIdempotencyKey(mock.Anything, order, domain.Reserve).Return(true, nil).Once()
		deps.repo.EXPECT().ReleaseStock(mock.Anything, int16(3), p1).Return(0, domain.ErrProductNotFound).Once()

		require.ErrorIs(t, s.ReleaseStock(ctx, reservation), domain.ErrProductNotFound)
	})

	t.Run("lock error", func(t *testing.T) {
		s, deps := setup(t)
		deps.repo.EXPECT().LockOrder(mock.Anything, order).Return(dbErr).Once()

		require.ErrorIs(t, s.ReleaseStock(ctx, reservation), dbErr)
	})

	t.Run("invalid input does not touch repository", func(t *testing.T) {
		s, _ := setup(t)

		require.ErrorIs(t, s.ReleaseStock(ctx, &domain.StockReservation{OrderID: order}), domain.ErrInvalidReservation)
	})
}
