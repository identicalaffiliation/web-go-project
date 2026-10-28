//go:build unit

package orders

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/orders/gen/mocks"
	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/dto"
)

type testDeps struct {
	repo     *mocks.MockRepository
	logger   *mocks.MockLogger
	manager  *mocks.MockTxManager
	prices   *mocks.MockPriceProvider
	workflow *mocks.MockWorkflowStarter
}

func setup(t *testing.T) (*Service, testDeps) {
	t.Helper()

	deps := testDeps{
		repo:     mocks.NewMockRepository(t),
		logger:   mocks.NewMockLogger(t),
		manager:  mocks.NewMockTxManager(t),
		prices:   mocks.NewMockPriceProvider(t),
		workflow: mocks.NewMockWorkflowStarter(t),
	}

	deps.logger.EXPECT().With(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().WithError(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().Error(mock.Anything, mock.Anything).Maybe()
	deps.manager.EXPECT().Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		}).Maybe()

	return NewService(deps.repo, deps.logger, deps.manager, deps.prices, deps.workflow), deps
}

func newCreateRequest() *dto.CreateRequest {
	return &dto.CreateRequest{
		ProductID:   uuid.NewV7(),
		OperationID: uuid.NewV7(),
		UserID:      uuid.NewV7(),
		Quantity:    3,
	}
}

// orderFromRequest - проверяет, что в репозиторий уходит заказ, собранный из запроса и цены.
func orderFromRequest(req *dto.CreateRequest, totalPrice int64) any {
	return mock.MatchedBy(func(o *domain.Order) bool {
		return o.ID != uuid.Nil() &&
			o.ProductID == req.ProductID &&
			o.UserID == req.UserID &&
			o.OperationID == req.OperationID &&
			o.Quantity == req.Quantity &&
			o.TotalPrice == totalPrice
	})
}

func TestService_CreateOrder(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("db down")

	t.Run("creates order and starts saga", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()
		created := &domain.Order{ID: uuid.NewV7(), Status: domain.Created, TotalPrice: 300}

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(100, nil).Once()
		deps.repo.EXPECT().CreateOrder(mock.Anything, orderFromRequest(req, 300)).Return(created, nil).Once()
		deps.workflow.EXPECT().StartOrderSaga(mock.Anything, created).Return(nil).Once()

		resp, err := s.CreateOrder(ctx, req)
		require.NoError(t, err)
		require.Equal(t, *created, resp.Order)
	})

	t.Run("price error stops before db", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()
		priceErr := errors.New("catalog unavailable")

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(0, priceErr).Once()

		_, err := s.CreateOrder(ctx, req)
		require.ErrorIs(t, err, priceErr)
		deps.repo.AssertNotCalled(t, "CreateOrder", mock.Anything, mock.Anything)
		deps.workflow.AssertNotCalled(t, "StartOrderSaga", mock.Anything, mock.Anything)
	})

	t.Run("duplicate operation id is rejected", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(100, nil).Once()
		deps.repo.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(nil, postgres.ErrAlreadyExists).Once()

		_, err := s.CreateOrder(ctx, req)
		require.ErrorIs(t, err, ErrOrderAlreadyExists)
		deps.workflow.AssertNotCalled(t, "StartOrderSaga", mock.Anything, mock.Anything)
	})

	t.Run("check constraint is invalid data", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(0, nil).Once()
		deps.repo.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(nil, postgres.ErrInvalidData).Once()

		_, err := s.CreateOrder(ctx, req)
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("db error", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(100, nil).Once()
		deps.repo.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(nil, dbErr).Once()

		_, err := s.CreateOrder(ctx, req)
		require.ErrorIs(t, err, dbErr)
	})

	t.Run("saga start error rolls back order", func(t *testing.T) {
		s, deps := setup(t)
		req := newCreateRequest()
		created := &domain.Order{ID: uuid.NewV7()}
		startErr := errors.New("temporal unavailable")

		deps.prices.EXPECT().GetPrice(mock.Anything, req.ProductID).Return(100, nil).Once()
		deps.repo.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(created, nil).Once()
		deps.workflow.EXPECT().StartOrderSaga(mock.Anything, created).Return(startErr).Once()

		// ошибка из транзакции - менеджер откатит insert
		_, err := s.CreateOrder(ctx, req)
		require.ErrorIs(t, err, startErr)
	})
}

func TestService_GetUserOrders(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid user id", func(t *testing.T) {
		s, _ := setup(t)

		_, err := s.GetUserOrders(ctx, "not-uuid")
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("returns orders", func(t *testing.T) {
		s, deps := setup(t)
		userID := uuid.NewV7()
		orders := []*domain.Order{
			{ID: uuid.NewV7(), UserID: userID, Status: domain.Paid},
			{ID: uuid.NewV7(), UserID: userID, Status: domain.Failed},
		}

		deps.repo.EXPECT().GetOrdersByUserID(mock.Anything, userID).Return(orders, nil).Once()

		resp, err := s.GetUserOrders(ctx, userID.String())
		require.NoError(t, err)
		require.Len(t, resp.GetOrders(), 2)
		require.Equal(t, orders[0].ID.String(), resp.GetOrders()[0].GetOrderId())
		require.Equal(t, ordersv1.OrderStatus_PAID, resp.GetOrders()[0].GetStatus())
		require.Equal(t, ordersv1.OrderStatus_FAILED, resp.GetOrders()[1].GetStatus())
	})

	t.Run("repo error", func(t *testing.T) {
		s, deps := setup(t)
		userID := uuid.NewV7()
		dbErr := errors.New("db down")

		deps.repo.EXPECT().GetOrdersByUserID(mock.Anything, userID).Return(nil, dbErr).Once()

		_, err := s.GetUserOrders(ctx, userID.String())
		require.ErrorIs(t, err, dbErr)
	})
}
