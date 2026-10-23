//go:build unit

package account

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

const initialBalance = 1_000_000

type testDeps struct {
	repo    *mocks.MockRepository
	logger  *mocks.MockLogger
	manager *mocks.MockTxManager
}

func setup(t *testing.T) (*Service, testDeps) {
	t.Helper()

	deps := testDeps{
		repo:    mocks.NewMockRepository(t),
		logger:  mocks.NewMockLogger(t),
		manager: mocks.NewMockTxManager(t),
	}

	deps.logger.EXPECT().With(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().WithError(mock.Anything).Return(deps.logger).Maybe()
	deps.logger.EXPECT().Error(mock.Anything).Maybe()
	deps.logger.EXPECT().Debug(mock.Anything).Maybe()
	deps.manager.EXPECT().Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		}).Maybe()

	return NewService(deps.repo, deps.logger, deps.manager, initialBalance), deps
}

func newEvent() *domain.UserCreated {
	return &domain.UserCreated{EventID: uuid.NewV7(), UserID: uuid.NewV7()}
}

func TestService_ProcessTask(t *testing.T) {
	ctx := context.Background()

	t.Run("opens account with initial balance", func(t *testing.T) {
		s, deps := setup(t)
		event := newEvent()

		deps.repo.EXPECT().SaveProcessedEvent(mock.Anything, event.EventID).Return(true, nil).Once()
		deps.repo.EXPECT().CreateAccount(mock.Anything, event.UserID, int64(initialBalance)).Return(true, nil).Once()

		require.NoError(t, s.ProcessTask(ctx, event))
	})

	t.Run("duplicate event is skipped", func(t *testing.T) {
		s, deps := setup(t)
		event := newEvent()

		deps.repo.EXPECT().SaveProcessedEvent(mock.Anything, event.EventID).Return(false, nil).Once()

		require.NoError(t, s.ProcessTask(ctx, event))
		deps.repo.AssertNotCalled(t, "CreateAccount", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("existing account is not credited again", func(t *testing.T) {
		s, deps := setup(t)
		event := newEvent()

		deps.repo.EXPECT().SaveProcessedEvent(mock.Anything, event.EventID).Return(true, nil).Once()
		deps.repo.EXPECT().CreateAccount(mock.Anything, event.UserID, int64(initialBalance)).Return(false, nil).Once()

		require.NoError(t, s.ProcessTask(ctx, event))
	})

	t.Run("save event error", func(t *testing.T) {
		s, deps := setup(t)
		event := newEvent()
		dbErr := errors.New("db down")

		deps.repo.EXPECT().SaveProcessedEvent(mock.Anything, event.EventID).Return(false, dbErr).Once()

		require.ErrorIs(t, s.ProcessTask(ctx, event), dbErr)
	})

	t.Run("create account error rolls back event", func(t *testing.T) {
		s, deps := setup(t)
		event := newEvent()
		dbErr := errors.New("db down")

		deps.repo.EXPECT().SaveProcessedEvent(mock.Anything, event.EventID).Return(true, nil).Once()
		deps.repo.EXPECT().CreateAccount(mock.Anything, event.UserID, int64(initialBalance)).Return(false, dbErr).Once()

		// ошибка из транзакции - менеджер откатит и event_id, событие обработается повторно
		require.ErrorIs(t, s.ProcessTask(ctx, event), dbErr)
	})
}
