//go:build integration

package integration

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/suite"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
)

func TestSagaSuite(t *testing.T) {
	suite.Run(t, new(SagaSuite))
}

func (s *SagaSuite) newRequest(price int64) *dto.CreateRequest {
	productID := uuid.NewV7()
	s.catalog.setPrice(productID, price)

	return &dto.CreateRequest{
		ProductID:   productID,
		OperationID: uuid.NewV7(),
		UserID:      uuid.NewV7(),
		Quantity:    3,
	}
}

func (s *SagaSuite) Test_CreateOrder_ReservedAndPaid() {
	req := s.newRequest(100)

	resp, err := s.service.CreateOrder(s.T().Context(), req)
	s.Require().NoError(err)
	s.Equal(domain.Created, resp.Order.Status)
	s.Equal(int64(300), resp.Order.TotalPrice)

	s.Require().NoError(s.waitSagaCompleted(resp.Order.ID))
	s.waitStatus(resp.Order.ID, domain.Paid)

	reserved, released := s.inventory.calls(resp.Order.ID)
	s.Equal(1, reserved)
	s.Equal(0, released)
}

func (s *SagaSuite) Test_CreateOrder_OutOfStock_CompensatedAndFailed() {
	req := s.newRequest(100)
	s.inventory.setOutOfStock(req.ProductID)

	resp, err := s.service.CreateOrder(s.T().Context(), req)
	s.Require().NoError(err)

	s.Require().NoError(s.waitSagaCompleted(resp.Order.ID))
	s.waitStatus(resp.Order.ID, domain.Failed)

	reserved, released := s.inventory.calls(resp.Order.ID)
	s.Equal(0, reserved)
	s.Equal(1, released)
}

// Повтор с тем же operation_id отклоняется: второго заказа и второй саги нет.
func (s *SagaSuite) Test_CreateOrder_DuplicateOperationID() {
	req := s.newRequest(100)

	first, err := s.service.CreateOrder(s.T().Context(), req)
	s.Require().NoError(err)

	_, err = s.service.CreateOrder(s.T().Context(), req)
	s.Require().ErrorIs(err, orders.ErrOrderAlreadyExists)

	s.Require().NoError(s.waitSagaCompleted(first.Order.ID))
	s.waitStatus(first.Order.ID, domain.Paid)
	s.Equal(1, s.ordersCount())
}

// Товара нет в catalog - заказ не создается и сага не стартует.
func (s *SagaSuite) Test_CreateOrder_UnknownProduct() {
	req := &dto.CreateRequest{
		ProductID:   uuid.NewV7(),
		OperationID: uuid.NewV7(),
		UserID:      uuid.NewV7(),
		Quantity:    1,
	}

	_, err := s.service.CreateOrder(s.T().Context(), req)
	s.Require().Error(err)
	s.Equal(0, s.ordersCount())
}
