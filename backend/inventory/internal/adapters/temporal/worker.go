package temporal

import (
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
)

type Worker struct {
	client client.Client
	worker worker.Worker
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

	w.RegisterActivityWithOptions(activities.ReserveStock, activity.RegisterOptions{Name: ReserveStockActivity})
	w.RegisterActivityWithOptions(activities.ReleaseStock, activity.RegisterOptions{Name: ReleaseStockActivity})

	return &Worker{client: c, worker: w}, nil
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
