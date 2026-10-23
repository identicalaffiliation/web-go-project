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
	env.RegisterActivityWithOptions(
		func(context.Context, uuid.UUID, domain.Status) error { return nil },
		activity.RegisterOptions{Name: UpdateStatusActivity},
	)

	return env
}

func sagaInput() SagaInput {
	return SagaInput{OrderID: uuid.NewV7(), ProductID: uuid.NewV7(), Quantity: 2}
}

func TestOrderSaga_Success(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Paid).Return(nil).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
	env.AssertNotCalled(t, ReleaseStockActivity, mock.Anything, mock.Anything)
}

func TestOrderSaga_InsufficientStock(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	reserveErr := sdktemporal.NewNonRetryableApplicationError("insufficient stock", "InsufficientStock", nil)
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(reserveErr).Once()
	env.OnActivity(ReleaseStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Failed).Return(nil).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestOrderSaga_OrderNotSaved(t *testing.T) {
	env := newSagaEnv(t)
	in := sagaInput()

	notFound := sdktemporal.NewNonRetryableApplicationError("not found", ErrTypeOrderNotFound, errors.New("not found"))
	env.OnActivity(ReserveStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Paid).Return(notFound).Once()
	env.OnActivity(ReleaseStockActivity, mock.Anything, in).Return(nil).Once()
	env.OnActivity(UpdateStatusActivity, mock.Anything, in.OrderID, domain.Failed).Return(notFound).Once()

	env.ExecuteWorkflow(OrderSaga, in)

	// сток возвращен, workflow падает с OrderNotFound - заказа в базе нет
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
