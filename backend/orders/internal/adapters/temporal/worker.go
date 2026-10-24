package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

type Worker struct {
	client    client.Client
	worker    worker.Worker
	taskQueue string
}

func NewWorker(cfg *config.TemporalConfig, activities *Activities) (*Worker, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}

	w := worker.New(c, cfg.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: cfg.MaxConcurrentActivities,
	})

	w.RegisterWorkflowWithOptions(OrderSaga, workflow.RegisterOptions{Name: OrderSagaWorkflow})
	w.RegisterActivityWithOptions(activities.UpdateOrderStatus, activity.RegisterOptions{Name: UpdateStatusActivity})

	return &Worker{client: c, worker: w, taskQueue: cfg.TaskQueue}, nil
}

// StartOrderSaga - запускает сагу заказа, один заказ - один workflow.
func (w *Worker) StartOrderSaga(ctx context.Context, order *domain.Order) error {
	_, err := w.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "order-" + order.ID.String(),
		TaskQueue: w.taskQueue,
	}, OrderSagaWorkflow, SagaInput{
		OrderID:    order.ID,
		ProductID:  order.ProductID,
		UserID:     order.UserID,
		Quantity:   order.Quantity,
		TotalPrice: order.TotalPrice,
	})
	if err != nil {
		return fmt.Errorf("start order saga: %w", err)
	}

	return nil
}

func (w *Worker) Run(stopCh <-chan struct{}) error {
	if err := w.worker.Run(toInterruptCh(stopCh)); err != nil {
		return fmt.Errorf("run temporal worker: %w", err)
	}

	return nil
}

func (w *Worker) Close() {
	w.client.Close()
}

func toInterruptCh(stopCh <-chan struct{}) <-chan any {
	ch := make(chan any)
	go func() {
		<-stopCh
		close(ch)
	}()

	return ch
}
