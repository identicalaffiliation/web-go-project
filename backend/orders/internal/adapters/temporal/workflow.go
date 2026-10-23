package temporal

import (
	"time"
	"uuid"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

const (
	OrderSagaWorkflow    = "orders.OrderSaga"
	UpdateStatusActivity = "orders.UpdateOrderStatus"

	ReserveStockActivity = "inventory.ReserveStock"
	ReleaseStockActivity = "inventory.ReleaseStock"
	InventoryTaskQueue   = "inventory"
)

const (
	inventoryActivityTimeout = 10 * time.Second
	ordersActivityTimeout    = 5 * time.Second
)

// SagaInput - входные данные саги заказа.
type SagaInput struct {
	OrderID   uuid.UUID `json:"orderId"`
	ProductID uuid.UUID `json:"productId"`
	Quantity  int16     `json:"quantity"`
}

// OrderSaga - сага заказа: резерв стока -> статус paid.
// При ошибке резерва или перевода в paid: компенсация ReleaseStock -> статус failed.
// пока нет оплаты
func OrderSaga(ctx workflow.Context, in SagaInput) error {
	inventoryCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           InventoryTaskQueue,
		StartToCloseTimeout: inventoryActivityTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	})

	// активити заказа выполняются на очереди самого workflow
	ordersCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: ordersActivityTimeout,
	})

	sagaErr := workflow.ExecuteActivity(inventoryCtx, ReserveStockActivity, in).Get(ctx, nil)
	if sagaErr == nil {
		// если заказ не сохранился (commit упал после старта саги) - резерв нужно вернуть
		sagaErr = workflow.ExecuteActivity(ordersCtx, UpdateStatusActivity, in.OrderID, domain.Paid).Get(ctx, nil)
		if sagaErr == nil {
			return nil
		}
	}

	workflow.GetLogger(ctx).Error("order saga failed, compensating",
		"order_id", in.OrderID.String(),
		"error", sagaErr,
	)

	return compensate(ctx, in)
}

// compensate - возвращает сток и переводит заказ в failed.
// Выполняется на отвязанном контексте, чтобы отработать даже при отмене workflow.
func compensate(ctx workflow.Context, in SagaInput) error {
	compCtx, _ := workflow.NewDisconnectedContext(ctx)

	releaseCtx := workflow.WithActivityOptions(compCtx, workflow.ActivityOptions{
		TaskQueue:           InventoryTaskQueue,
		StartToCloseTimeout: inventoryActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumInterval: time.Minute},
	})

	if err := workflow.ExecuteActivity(releaseCtx, ReleaseStockActivity, in).Get(compCtx, nil); err != nil {
		return err
	}

	ordersCtx := workflow.WithActivityOptions(compCtx, workflow.ActivityOptions{
		StartToCloseTimeout: ordersActivityTimeout,
	})

	return workflow.ExecuteActivity(ordersCtx, UpdateStatusActivity, in.OrderID, domain.Failed).Get(compCtx, nil)
}
