package orders

import (
	"context"
	"errors"
	"uuid"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/ports"
	"go.uber.org/zap"
)

var (
	ErrInvalidData        = errors.New("invalid data")
	ErrOrderAlreadyExists = errors.New("order already exists")
)

type Service struct {
	repo     ports.Repository
	logger   ports.Logger
	manager  ports.TxManager
	prices   ports.PriceProvider
	workflow ports.WorkflowStarter
}

func NewService(
	repo ports.Repository,
	logger ports.Logger,
	manager ports.TxManager,
	prices ports.PriceProvider,
	workflow ports.WorkflowStarter,
) *Service {
	return &Service{repo: repo, logger: logger, manager: manager, prices: prices, workflow: workflow}
}

// CreateOrder - создает заказ со статусом created и запускает сагу заказа.
// operation_id - ключ идемпотентности: повторный запрос с тем же ключом отклоняется.
// Сага стартует внутри транзакции: если workflow не запустился - заказ не сохранится.
func (s *Service) CreateOrder(ctx context.Context, req *dto.CreateRequest) (*dto.OrderResponse, error) {
	logger := s.logger.With(
		zap.String("operation_id", req.OperationID.String()),
		zap.String("user_id", req.UserID.String()),
	)

	price, err := s.prices.GetPrice(ctx, req.ProductID)
	if err != nil {
		logger.WithError(err).Error("failed to get product price")
		return nil, err
	}

	order := req.ToDomain()
	order.TotalPrice = price * int64(order.Quantity)

	var created *domain.Order
	err = s.manager.Do(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repo.CreateOrder(ctx, order)
		if err != nil {
			return err
		}

		return s.workflow.StartOrderSaga(ctx, created)
	})

	switch {
	case errors.Is(err, postgres.ErrAlreadyExists):
		return nil, ErrOrderAlreadyExists
	case errors.Is(err, postgres.ErrInvalidData):
		return nil, ErrInvalidData
	case err != nil:
		logger.WithError(err).Error("failed to create order")
		return nil, err
	}

	return dto.NewOrderResponse(created), nil
}

func (s *Service) GetUserOrders(ctx context.Context, id string) (*ordersv1.UserOrdersResponse, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidData
	}

	logger := s.logger.With(zap.String("user_id", id))

	orders, err := s.repo.GetOrdersByUserID(ctx, userID)
	if err != nil {
		logger.WithError(err).Error("failed to get user orders")
		return nil, err
	}

	response := make([]*ordersv1.UserOrderResponse, len(orders))
	for i := range orders {
		response[i] = orders[i].ToResponseRPC()
	}

	return &ordersv1.UserOrdersResponse{Orders: response}, nil
}
