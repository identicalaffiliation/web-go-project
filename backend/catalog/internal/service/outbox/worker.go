package outbox

import (
	"context"
	"sync"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
)

func Run(
	ctx context.Context,
	repo ports.OutboxRepository,
	producer ports.Producer,
	logger ports.Logger,
	cfg *config.OutboxConfig,
	wg *sync.WaitGroup,
) error {
	ticker := time.NewTicker(cfg.TickTime)
	defer wg.Done()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := processEvents(ctx, repo, producer, cfg.Limit); err != nil {
				logger.WithError(err).Error("failed to sent events")
			}
		}
	}
}

func processEvents(ctx context.Context, repo ports.OutboxRepository, producer ports.Producer, limit int64) error {
	events, err := repo.GetEvents(ctx, limit)
	if err != nil {
		return err
	}

	if err := producer.SendMessages(ctx, events); err != nil {
		return err
	}

	ids := make([]uuid.UUID, len(events))
	for i, v := range events {
		ids[i] = v.EventID
	}

	if err := repo.SentEvent(ctx, ids); err != nil {
		return err
	}

	return nil
}
