package workflow

import (
	"context"

	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
)

// Service - бизнес-логика шагов саги заказа, вызывается temporal активити.
// Оба метода идемпотентны: активити может ретраиться и приходить повторно.
type Service struct {
	repo    ports.Repository
	logger  ports.Logger
	manager ports.TxManager
}

func NewService(repo ports.Repository, logger ports.Logger, manager ports.TxManager) *Service {
	return &Service{repo: repo, logger: logger, manager: manager}
}

// Charge - списывает стоимость заказа с баланса пользователя.
// Списание и запись в журнал операций - одна транзакция: либо деньги списаны и это записано,
// либо ничего не произошло. Повтор по тому же заказу ничего не списывает.
func (s *Service) Charge(ctx context.Context, payment *domain.Payment) error {
	if err := payment.Validate(); err != nil {
		return err
	}

	logger := s.logger.With(
		zap.String("order_id", payment.OrderID.String()),
		zap.String("user_id", payment.UserID.String()),
	)

	return s.manager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockOrder(ctx, payment.OrderID); err != nil {
			logger.WithError(err).Error("failed to lock order")
			return err
		}

		_, charged, err := s.repo.GetOperation(ctx, payment.OrderID, domain.Charge)
		if err != nil {
			logger.WithError(err).Error("failed to check charge operation")
			return err
		}

		// order already charged
		if charged {
			return nil
		}

		// компенсация уже отработала (например, списание упало по таймауту и сага откатилась),
		// запоздавшее списание применять нельзя - деньги больше никто не вернет
		_, refunded, err := s.repo.GetOperation(ctx, payment.OrderID, domain.Refund)
		if err != nil {
			logger.WithError(err).Error("failed to check refund operation")
			return err
		}

		if refunded {
			return domain.ErrOrderRefunded
		}

		if _, err := s.repo.Debit(ctx, payment.UserID, payment.Amount); err != nil {
			logger.WithError(err).Error("failed to debit account")
			return err
		}

		return s.repo.SaveOperation(ctx, &domain.PaymentOperation{
			OrderID:   payment.OrderID,
			Operation: domain.Charge,
			UserID:    payment.UserID,
			Amount:    payment.Amount,
		})
	})
}

// Refund - компенсация списания. Возвращает ровно ту сумму, что была списана по заказу,
// и только если списание было. Запись возврата сохраняется в любом случае,
// чтобы запоздавшее списание не применилось после компенсации.
func (s *Service) Refund(ctx context.Context, payment *domain.Payment) error {
	if err := payment.Validate(); err != nil {
		return err
	}

	logger := s.logger.With(
		zap.String("order_id", payment.OrderID.String()),
		zap.String("user_id", payment.UserID.String()),
	)

	return s.manager.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockOrder(ctx, payment.OrderID); err != nil {
			logger.WithError(err).Error("failed to lock order")
			return err
		}

		_, refunded, err := s.repo.GetOperation(ctx, payment.OrderID, domain.Refund)
		if err != nil {
			logger.WithError(err).Error("failed to check refund operation")
			return err
		}

		// order already refunded
		if refunded {
			return nil
		}

		charge, charged, err := s.repo.GetOperation(ctx, payment.OrderID, domain.Charge)
		if err != nil {
			logger.WithError(err).Error("failed to check charge operation")
			return err
		}

		refund := &domain.PaymentOperation{
			OrderID:   payment.OrderID,
			Operation: domain.Refund,
			UserID:    payment.UserID,
		}

		if charged {
			if _, err := s.repo.Credit(ctx, charge.UserID, charge.Amount); err != nil {
				logger.WithError(err).Error("failed to credit account")
				return err
			}

			refund.UserID = charge.UserID
			refund.Amount = charge.Amount
		}

		return s.repo.SaveOperation(ctx, refund)
	})
}
