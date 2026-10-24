package account

import (
	"context"

	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
)

// Service - открытие счета по событию регистрации пользователя.
type Service struct {
	repo           ports.Repository
	logger         ports.Logger
	manager        ports.TxManager
	initialBalance int64
}

func NewService(repo ports.Repository, logger ports.Logger, manager ports.TxManager, initialBalance int64) *Service {
	return &Service{repo: repo, logger: logger, manager: manager, initialBalance: initialBalance}
}

// ProcessTask - идемпотентно открывает счет со стартовым балансом.
// event_id и счет пишутся в одной транзакции: повторная доставка события (ребаланс группы,
// падение до коммита offset) ничего не меняет. Счет уникален по user_id, поэтому и разные
// события об одном пользователе не начислят деньги дважды.
func (s *Service) ProcessTask(ctx context.Context, event *domain.UserCreated) error {
	logger := s.logger.With(
		zap.String("event_id", event.EventID.String()),
		zap.String("user_id", event.UserID.String()),
	)

	return s.manager.Do(ctx, func(ctx context.Context) error {
		isNew, err := s.repo.SaveProcessedEvent(ctx, event.EventID)
		if err != nil {
			logger.WithError(err).Error("failed to save processed event")
			return err
		}

		if !isNew {
			logger.Debug("event already processed, skipping")
			return nil
		}

		created, err := s.repo.CreateAccount(ctx, event.UserID, s.initialBalance)
		if err != nil {
			logger.WithError(err).Error("failed to create account")
			return err
		}

		if !created {
			logger.Debug("account already exists, skipping")
		}

		return nil
	})
}
