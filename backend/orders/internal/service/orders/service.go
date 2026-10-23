package orders

import (
	"context"
	"errors"
	"uuid"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/ports"
	"go.uber.org/zap"
)

var ErrInvalidData = errors.New("invalid data")

type Service struct {
	repo    ports.Repository
	logger  ports.Logger
	manager ports.TxManager
}

func NewService(repo ports.Repository, logger ports.Logger, manager ports.TxManager) *Service {
	return &Service{repo: repo, logger: logger, manager: manager}
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
