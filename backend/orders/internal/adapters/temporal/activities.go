package temporal

import (
	"context"
	"errors"
	"uuid"

	"go.temporal.io/sdk/temporal"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

const (
	ErrTypeOrderNotFound = "OrderNotFound"
)

type statusUpdater interface {
	UpdateStatusByOrderID(ctx context.Context, id uuid.UUID, newStatus domain.Status) error
}

type Activities struct {
	repo statusUpdater
}

func NewActivities(repo statusUpdater) *Activities {
	return &Activities{repo: repo}
}

func (a *Activities) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	err := a.repo.UpdateStatusByOrderID(ctx, id, status)
	if errors.Is(err, postgres.ErrNotFound) {
		return temporal.NewNonRetryableApplicationError(err.Error(), ErrTypeOrderNotFound, err)
	}

	return err
}
