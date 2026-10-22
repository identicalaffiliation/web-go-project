package task_consumer

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
)

type TaskProcesser struct {
	repo ports.StockRepository
}

func NewTaskProcesser(repo ports.StockRepository) *TaskProcesser {
	return &TaskProcesser{
		repo: repo,
	}
}

func (tp *TaskProcesser) ProcessTask(ctx context.Context, product *domain.Product) error {
	return tp.repo.CreateStock(ctx, domain.FromDomainProductToStock(product))
}
