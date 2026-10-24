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

	ChargeActivity    = "payments.Charge"
	RefundActivity    = "payments.Refund"
	PaymentsTaskQueue = "payments"
)

const (
	inventoryActivityTimeout = 10 * time.Second
	paymentsActivityTimeout  = 10 * time.Second
	ordersActivityTimeout    = 5 * time.Second
)

// SagaInput - входные данные саги заказа.
type SagaInput struct {
	OrderID    uuid.UUID `json:"orderId"`
	ProductID  uuid.UUID `json:"productId"`
	UserID     uuid.UUID `json:"userId"`
	Quantity   int16     `json:"quantity"`
	TotalPrice int64     `json:"totalPrice"`
}

// PaymentInput - вход активити payments (payments/internal/domain.Payment).
type PaymentInput struct {
	OrderID uuid.UUID `json:"orderId"`
	UserID  uuid.UUID `json:"userId"`
	Amount  int64     `json:"amount"`
}

func (in SagaInput) payment() PaymentInput {
	return PaymentInput{OrderID: in.OrderID, UserID: in.UserID, Amount: in.TotalPrice}
}

// retryPolicy - ретраи внешних шагов саги. Бизнес-ошибки (нет стока, нет денег) non retryable
// и сразу уводят сагу в компенсацию.
func retryPolicy() *temporal.RetryPolicy {
	return &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2,
		MaximumInterval:    30 * time.Second,
		MaximumAttempts:    5,
	}
}

// OrderSaga - сага заказа: резерв стока -> списание денег -> статус paid.
// При ошибке любого шага: компенсация Refund -> ReleaseStock -> статус failed.
func OrderSaga(ctx workflow.Context, in SagaInput) error {
	inventoryCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           InventoryTaskQueue,
		StartToCloseTimeout: inventoryActivityTimeout,
		RetryPolicy:         retryPolicy(),
	})

	paymentsCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           PaymentsTaskQueue,
		StartToCloseTimeout: paymentsActivityTimeout,
		RetryPolicy:         retryPolicy(),
	})

	// активити заказа выполняются на очереди самого workflow
	ordersCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: ordersActivityTimeout,
	})

	sagaErr := workflow.ExecuteActivity(inventoryCtx, ReserveStockActivity, in).Get(ctx, nil)
	if sagaErr == nil {
		sagaErr = workflow.ExecuteActivity(paymentsCtx, ChargeActivity, in.payment()).Get(ctx, nil)
	}

	if sagaErr == nil {
		// если заказ не сохранился (commit упал после старта саги) - деньги и резерв нужно вернуть
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

// compensate - возвращает деньги и сток, переводит заказ в failed.
// Refund и ReleaseStock вызываются всегда: оба идемпотентны и ничего не делают, если шаг не применился,
// а их запись блокирует запоздавшее списание/резерв.
// Выполняется на отвязанном контексте, чтобы отработать даже при отмене workflow.
func compensate(ctx workflow.Context, in SagaInput) error {
	compCtx, _ := workflow.NewDisconnectedContext(ctx)

	refundCtx := workflow.WithActivityOptions(compCtx, workflow.ActivityOptions{
		TaskQueue:           PaymentsTaskQueue,
		StartToCloseTimeout: paymentsActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumInterval: time.Minute},
	})

	if err := workflow.ExecuteActivity(refundCtx, RefundActivity, in.payment()).Get(compCtx, nil); err != nil {
		return err
	}

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
