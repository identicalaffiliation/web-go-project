//go:build unit

package temporal

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

func newSagaEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	// заглушки активити inventory, в тестах переопределяются через OnActivity
	env.RegisterActivityWithOptions(
		func(context.Context, SagaInput) error { return nil },
		activity.RegisterOptions{Name: ReserveStockActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, SagaInput) error { return nil },
		activity.RegisterOptions{Name: ReleaseStockActivity},
	)
	// заглушки активити payments
	env.RegisterActivityWithOptions(
		func(context.Context, PaymentInput) error { return nil },
		activity.RegisterOptions{Name: ChargeActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, PaymentInput) error { return nil },
		activity.RegisterOptions{Name: RefundActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, uuid.UUID, domain.Status) error { return nil },
		activity.RegisterOptions{Name: UpdateStatusActivity},
	)

	return env
}

func sagaInput() SagaInput {
	return SagaInput{
		OrderID:    uuid.NewV7(),
		ProductID:  uuid.NewV7(),
		UserID:     uuid.NewV7(),
		Quantity:   2,
		TotalPrice: 200,
	}
}

func paymentInput(in SagaInput) PaymentInput {
	return PaymentInput{OrderID: in.OrderID, UserID: in.UserID, Amount: in.TotalPrice}
}

func TestOrderSaga_Success(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(ChargeActivity, mock.Anything, paymentInput(in)).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Paid).Return(nil).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
	env.AssertNotCalled(t, RefundActivity, mock.Anything, mock.Anything)
	env.AssertNotCalled(t, ReleaseStockActivity, mock.Anything, mock.Anything)
}

func TestOrderSaga_InsufficientStock(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	reserveErr := sdktemporal.NewNonRetryableApplicationError("insufficient stock", "InsufficientStock", nil)
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(reserveErr).Once()
	env.OnActivity(RefundActivity, mock.Anything, paymentInput(in)).Return(nil).Once()
	env.OnActivity(ReleaseStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Failed).Return(nil).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
	env.AssertNotCalled(t, ChargeActivity, mock.Anything, mock.Anything)
}

func TestOrderSaga_InsufficientFunds(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	chargeErr := sdktemporal.NewNonRetryableApplicationError("insufficient funds", "InsufficientFunds", nil)
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(ChargeActivity, mock.Anything, paymentInput(in)).Return(chargeErr).Once()
	env.OnActivity(RefundActivity, mock.Anything, paymentInput(in)).Return(nil).Once()
	env.OnActivity(ReleaseStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Failed).Return(nil).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	// сток возвращен, заказ failed, paid не выставлялся
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
	env.AssertNotCalled(t, UpdateStatusActivity, mock.Anything, in.OrderID, domain.Paid)
}

func TestOrderSaga_RefundFails(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	reserveErr := sdktemporal.NewNonRetryableApplicationError("insufficient stock", "InsufficientStock", nil)
	refundErr := sdktemporal.NewNonRetryableApplicationError("account not found", "AccountNotFound", nil)
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(reserveErr).Once()
	env.OnActivity(RefundActivity, mock.Anything, paymentInput(in)).Return(refundErr).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	// компенсация не прошла - workflow падает, чтобы это было видно в temporal
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
	env.AssertNotCalled(t, ReleaseStockActivity, mock.Anything, mock.Anything)
}

func TestOrderSaga_OrderNotSaved(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	notFound := sdktemporal.NewNonRetryableApplicationError("not found", ErrTypeOrderNotFound, errors.New("not found"))
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(ChargeActivity, mock.Anything, paymentInput(in)).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Paid).Return(notFound).Once()
	env.OnActivity(RefundActivity, mock.Anything, paymentInput(in)).Return(nil).Once()
	env.OnActivity(ReleaseStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Failed).Return(notFound).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	// деньги и сток возвращены, workflow падает с OrderNotFound - заказа в базе нет
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
