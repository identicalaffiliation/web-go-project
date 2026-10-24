//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/stretchr/testify/suite"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositorySuite))
}

func (s *RepositorySuite) Test_CreateOrder_DefaultStatusCreated() {
	order := newOrder(uuid.NewV7())

	created := s.createOrder(order)

	s.Equal(order.ID, created.ID)
	s.Equal(order.UserID, created.UserID)
	s.Equal(order.ProductID, created.ProductID)
	s.Equal(order.OperationID, created.OperationID)
	s.Equal(order.Quantity, created.Quantity)
	s.Equal(order.TotalPrice, created.TotalPrice)
	s.Equal(domain.Created, created.Status)
	s.False(created.CreatedAt.IsZero())
	s.False(created.UpdatedAt.IsZero())
}

// operation_id - ключ идемпотентности, второй заказ с тем же ключом не создается.
func (s *RepositorySuite) Test_CreateOrder_DuplicateOperationID() {
	order := newOrder(uuid.NewV7())
	s.createOrder(order)

	duplicate := newOrder(order.UserID)
	duplicate.OperationID = order.OperationID

	_, err := s.repo.CreateOrder(s.T().Context(), duplicate)
	s.Require().ErrorIs(err, postgres.ErrAlreadyExists)
	s.Equal(1, s.ordersCount())
}

func (s *RepositorySuite) Test_CreateOrder_CheckConstraints() {
	cases := map[string]func(o *domain.Order){
		"zero quantity":    func(o *domain.Order) { o.Quantity = 0 },
		"zero total price": func(o *domain.Order) { o.TotalPrice = 0 },
	}

	for name, modify := range cases {
		s.Run(name, func() {
			order := newOrder(uuid.NewV7())
			modify(order)

			_, err := s.repo.CreateOrder(s.T().Context(), order)
			s.Require().ErrorIs(err, postgres.ErrInvalidData)
		})
	}

	s.Equal(0, s.ordersCount())
}

// Ошибка внутри транзакции (как падение старта саги) откатывает insert.
func (s *RepositorySuite) Test_CreateOrder_RollbackInTransaction() {
	trm := manager.Must(pgxv5.NewDefaultFactory(s.pool))
	startErr := errors.New("saga start failed")

	err := trm.Do(s.T().Context(), func(ctx context.Context) error {
		if _, err := s.repo.CreateOrder(ctx, newOrder(uuid.NewV7())); err != nil {
			return err
		}

		return startErr
	})

	s.Require().ErrorIs(err, startErr)
	s.Equal(0, s.ordersCount())
}

func (s *RepositorySuite) Test_GetOrdersByUserID_NewestFirst() {
	userID := uuid.NewV7()
	now := time.Now().UTC()

	oldest := s.createOrder(newOrder(userID))
	middle := s.createOrder(newOrder(userID))
	newest := s.createOrder(newOrder(userID))
	s.setCreatedAt(oldest.ID, now.Add(-2*time.Hour))
	s.setCreatedAt(middle.ID, now.Add(-time.Hour))
	s.setCreatedAt(newest.ID, now)

	// чужой заказ не попадает в выборку
	s.createOrder(newOrder(uuid.NewV7()))

	orders, err := s.repo.GetOrdersByUserID(s.T().Context(), userID)
	s.Require().NoError(err)
	s.Require().Len(orders, 3)
	s.Equal(newest.ID, orders[0].ID)
	s.Equal(middle.ID, orders[1].ID)
	s.Equal(oldest.ID, orders[2].ID)
}

func (s *RepositorySuite) Test_GetOrdersByUserID_Limit30() {
	userID := uuid.NewV7()
	now := time.Now().UTC()

	var latest uuid.UUID
	for i := range 35 {
		order := s.createOrder(newOrder(userID))
		s.setCreatedAt(order.ID, now.Add(time.Duration(i)*time.Minute))
		latest = order.ID
	}

	orders, err := s.repo.GetOrdersByUserID(s.T().Context(), userID)
	s.Require().NoError(err)
	s.Len(orders, 30)
	s.Equal(latest, orders[0].ID)
}

func (s *RepositorySuite) Test_GetOrdersByUserID_Empty() {
	orders, err := s.repo.GetOrdersByUserID(s.T().Context(), uuid.NewV7())
	s.Require().NoError(err)
	s.Empty(orders)
}

func (s *RepositorySuite) Test_UpdateStatusByOrderID() {
	order := s.createOrder(newOrder(uuid.NewV7()))

	s.Require().NoError(s.repo.UpdateStatusByOrderID(s.T().Context(), order.ID, domain.Paid))

	status, updatedAt := s.orderStatus(order.ID)
	s.Equal(domain.Paid, status)
	s.False(updatedAt.Before(order.UpdatedAt))
}

func (s *RepositorySuite) Test_UpdateStatusByOrderID_NotFound() {
	err := s.repo.UpdateStatusByOrderID(s.T().Context(), uuid.NewV7(), domain.Failed)
	s.Require().ErrorIs(err, postgres.ErrNotFound)
}
