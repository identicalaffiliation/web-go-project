//go:build unit

package workflow

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
	deps.manager.EXPECT().Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		}).Maybe()

	return NewService(deps.repo, deps.logger, deps.manager), deps
}

func newPayment() *domain.Payment {
	return &domain.Payment{OrderID: uuid.NewV7(), UserID: uuid.NewV7(), Amount: 300}
}

func operation(p *domain.Payment, op domain.Operation, amount int64) *domain.PaymentOperation {
	return &domain.PaymentOperation{OrderID: p.OrderID, Operation: op, UserID: p.UserID, Amount: amount}
}

func TestService_Charge(t *testing.T) {
	ctx := context.Background()

	t.Run("debits and saves charge", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).Return(nil, false, nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).Return(nil, false, nil).Once()
		deps.repo.EXPECT().Debit(mock.Anything, p.UserID, p.Amount).Return(999_700, nil).Once()
		deps.repo.EXPECT().SaveOperation(mock.Anything, operation(p, domain.Charge, p.Amount)).Return(nil).Once()

		require.NoError(t, s.Charge(ctx, p))
	})

	t.Run("already charged is noop", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).
			Return(operation(p, domain.Charge, p.Amount), true, nil).Once()

		require.NoError(t, s.Charge(ctx, p))
		deps.repo.AssertNotCalled(t, "Debit", mock.Anything, mock.Anything, mock.Anything)
		deps.repo.AssertNotCalled(t, "SaveOperation", mock.Anything, mock.Anything)
	})

	t.Run("charge after refund is rejected", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).Return(nil, false, nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).
			Return(operation(p, domain.Refund, 0), true, nil).Once()

		require.ErrorIs(t, s.Charge(ctx, p), domain.ErrOrderRefunded)
		deps.repo.AssertNotCalled(t, "Debit", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, mock.Anything).Return(nil, false, nil).Twice()
		deps.repo.EXPECT().Debit(mock.Anything, p.UserID, p.Amount).Return(0, domain.ErrInsufficientFunds).Once()

		require.ErrorIs(t, s.Charge(ctx, p), domain.ErrInsufficientFunds)
		deps.repo.AssertNotCalled(t, "SaveOperation", mock.Anything, mock.Anything)
	})

	t.Run("invalid payment", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()
		p.Amount = 0

		require.ErrorIs(t, s.Charge(ctx, p), domain.ErrInvalidPayment)
		deps.manager.AssertNotCalled(t, "Do", mock.Anything, mock.Anything)
	})

	t.Run("lock error", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()
		lockErr := errors.New("db down")

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(lockErr).Once()

		require.ErrorIs(t, s.Charge(ctx, p), lockErr)
	})

	t.Run("save operation error is returned to roll back debit", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()
		saveErr := errors.New("db down")

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, mock.Anything).Return(nil, false, nil).Twice()
		deps.repo.EXPECT().Debit(mock.Anything, p.UserID, p.Amount).Return(999_700, nil).Once()
		deps.repo.EXPECT().SaveOperation(mock.Anything, mock.Anything).Return(saveErr).Once()

		require.ErrorIs(t, s.Charge(ctx, p), saveErr)
	})
}

func TestService_Refund(t *testing.T) {
	ctx := context.Background()

	t.Run("credits charged amount", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()
		// сумма возврата берется из журнала, а не из входа активити
		charge := operation(p, domain.Charge, 250)

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).Return(nil, false, nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).Return(charge, true, nil).Once()
		deps.repo.EXPECT().Credit(mock.Anything, p.UserID, int64(250)).Return(1_000_000, nil).Once()
		deps.repo.EXPECT().SaveOperation(mock.Anything, operation(p, domain.Refund, 250)).Return(nil).Once()

		require.NoError(t, s.Refund(ctx, p))
	})

	t.Run("not charged saves refund marker only", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).Return(nil, false, nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).Return(nil, false, nil).Once()
		deps.repo.EXPECT().SaveOperation(mock.Anything, operation(p, domain.Refund, 0)).Return(nil).Once()

		require.NoError(t, s.Refund(ctx, p))
		deps.repo.AssertNotCalled(t, "Credit", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("already refunded is noop", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).
			Return(operation(p, domain.Refund, p.Amount), true, nil).Once()

		require.NoError(t, s.Refund(ctx, p))
		deps.repo.AssertNotCalled(t, "Credit", mock.Anything, mock.Anything, mock.Anything)
		deps.repo.AssertNotCalled(t, "SaveOperation", mock.Anything, mock.Anything)
	})

	t.Run("credit error", func(t *testing.T) {
		s, deps := setup(t)
		p := newPayment()
		creditErr := errors.New("db down")

		deps.repo.EXPECT().LockOrder(mock.Anything, p.OrderID).Return(nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Refund).Return(nil, false, nil).Once()
		deps.repo.EXPECT().GetOperation(mock.Anything, p.OrderID, domain.Charge).
			Return(operation(p, domain.Charge, p.Amount), true, nil).Once()
		deps.repo.EXPECT().Credit(mock.Anything, p.UserID, p.Amount).Return(0, creditErr).Once()

		require.ErrorIs(t, s.Refund(ctx, p), creditErr)
		deps.repo.AssertNotCalled(t, "SaveOperation", mock.Anything, mock.Anything)
	})

	t.Run("invalid payment", func(t *testing.T) {
		s, deps := setup(t)

		require.ErrorIs(t, s.Refund(ctx, &domain.Payment{}), domain.ErrInvalidPayment)
		deps.manager.AssertNotCalled(t, "Do", mock.Anything, mock.Anything)
	})
}
