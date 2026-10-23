package balance

import (
	"context"
	"errors"
	"uuid"

	"go.uber.org/zap"

	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
)

var ErrInvalidData = errors.New("invalid data")

type Service struct {
	repo   ports.Repository
	logger ports.Logger
}

func NewService(repo ports.Repository, logger ports.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) GetBalance(ctx context.Context, id string) (*balancev1.GetBalanceResponse, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrInvalidData
	}

	account, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		if !errors.Is(err, domain.ErrAccountNotFound) {
			s.logger.With(zap.String("user_id", id)).WithError(err).Error("failed to get account")
		}

		return nil, err
	}

	return account.ToResponseRPC(), nil
}
