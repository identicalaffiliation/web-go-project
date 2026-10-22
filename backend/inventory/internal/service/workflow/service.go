package workflow

import (
	"context"

	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
)

// Service - бизнес-логика шагов саги заказа, вызывается temporal активити.
// Оба метода идемпотентны: активити может ретраиться и приходить повторно.
type Service struct {
	repo    ports.StockRepository
	logger  ports.Logger
	manager ports.TxManager
}

func NewService(repo ports.StockRepository, logger ports.Logger, manager ports.TxManager) *Service {
	return &Service{
		repo:    repo,
		logger:  logger,
		manager: manager,
	}
}

// ReserveStock - списывает остатки по всем позициям заказа в одной транзакции.
func (s *Service) ReserveStock(ctx context.Context, reservation *domain.StockReservation) error {
	if err := reservation.Validate(); err != nil {
		return err
	}

	logger := s.logger.With(
		zap.String("order_id", reservation.OrderID.String()),
		zap.String("product_id", reservation.ProductID.String()),
	)

	return s.manager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockOrder(ctx, reservation.OrderID); err != nil {
			logger.WithError(err).Error("failed to lock order")
			return err
		}

		reserved, err := s.repo.CheckIdempotencyKey(ctx, reservation.OrderID, domain.Reserve)
		if err != nil {
			logger.WithError(err).Error("failed to check reserve idempotency key")
			return err
		}

		// stock already reserved
		if reserved {
			return nil
		}

		// компенсация уже отработала (например, резерв упал по таймауту и сага откатилась),
		// запоздавший резерв применять нельзя - сток больше никто не вернет
		released, err := s.repo.CheckIdempotencyKey(ctx, reservation.OrderID, domain.Release)
		if err != nil {
			logger.WithError(err).Error("failed to check release idempotency key")
			return err
		}

		if released {
			return domain.ErrOrderReleased
		}

		if _, err := s.repo.ReserveStock(ctx, reservation.Quantity, reservation.ProductID); err != nil {
			logger.WithError(err).Error("failed to reserve stock")
			return err
		}

		return s.repo.SaveIdempotencyKey(ctx, reservation.OrderID, domain.Reserve)
	})
}

// ReleaseStock - компенсация резерва. Возвращает сток только если резерв по заказу был применен.
// Ключ релиза сохраняется в любом случае, чтобы запоздавший резерв не применился после компенсации.
func (s *Service) ReleaseStock(ctx context.Context, reservation *domain.StockReservation) error {
	if err := reservation.Validate(); err != nil {
		return err
	}

	logger := s.logger.With(
		zap.String("order_id", reservation.OrderID.String()),
		zap.String("product_id", reservation.ProductID.String()),
	)

	return s.manager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockOrder(ctx, reservation.OrderID); err != nil {
			logger.WithError(err).Error("failed to lock order")
			return err
		}

		released, err := s.repo.CheckIdempotencyKey(ctx, reservation.OrderID, domain.Release)
		if err != nil {
			logger.WithError(err).Error("failed to check release idempotency key")
			return err
		}

		// stock already released
		if released {
			return nil
		}

		reserved, err := s.repo.CheckIdempotencyKey(ctx, reservation.OrderID, domain.Reserve)
		if err != nil {
			logger.WithError(err).Error("failed to check reserve idempotency key")
			return err
		}

		if reserved {
			if _, err := s.repo.ReleaseStock(ctx, reservation.Quantity, reservation.ProductID); err != nil {
				logger.WithError(err).Error("failed to release stock")
				return err
			}
		}

		return s.repo.SaveIdempotencyKey(ctx, reservation.OrderID, domain.Release)
	})
}
