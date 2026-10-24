//go:build unit

package balance

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/payments/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

func setup(t *testing.T) (*Service, *mocks.MockRepository) {
	t.Helper()

	repo := mocks.NewMockRepository(t)
	logger := mocks.NewMockLogger(t)
	logger.EXPECT().With(mock.Anything).Return(logger).Maybe()
	logger.EXPECT().WithError(mock.Anything).Return(logger).Maybe()
	logger.EXPECT().Error(mock.Anything).Maybe()

	return NewService(repo, logger), repo
}

func TestService_GetBalance(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid user id", func(t *testing.T) {
		s, _ := setup(t)

		_, err := s.GetBalance(ctx, "not-uuid")
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("returns balance", func(t *testing.T) {
		s, repo := setup(t)
		userID := uuid.NewV7()

		repo.EXPECT().GetAccount(mock.Anything, userID).
			Return(&domain.Account{UserID: userID, Balance: 1_000_000}, nil).Once()

		resp, err := s.GetBalance(ctx, userID.String())
		require.NoError(t, err)
		require.Equal(t, userID.String(), resp.GetUserId())
		require.Equal(t, int64(1_000_000), resp.GetAmount())
	})

	t.Run("account not found", func(t *testing.T) {
		s, repo := setup(t)
		userID := uuid.NewV7()

		repo.EXPECT().GetAccount(mock.Anything, userID).Return(nil, domain.ErrAccountNotFound).Once()

		_, err := s.GetBalance(ctx, userID.String())
		require.ErrorIs(t, err, domain.ErrAccountNotFound)
	})

	t.Run("repo error", func(t *testing.T) {
		s, repo := setup(t)
		userID := uuid.NewV7()
		dbErr := errors.New("db down")

		repo.EXPECT().GetAccount(mock.Anything, userID).Return(nil, dbErr).Once()

		_, err := s.GetBalance(ctx, userID.String())
		require.ErrorIs(t, err, dbErr)
	})
}
