package outbox

import (
	"context"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
)

// Run - раз в TickTime отправляет пачку неотправленных событий в kafka.
// Безопасен при нескольких репликах: выборка, отправка и отметка sent_at идут в одной
// транзакции, а строки, занятые другой репликой, пропускаются (FOR UPDATE SKIP LOCKED).
func Run(
	ctx context.Context,
	manager ports.TxManager,
	repo ports.OutboxRepository,
	producer ports.Producer,
	logger ports.Logger,
	cfg *config.OutboxWorkerConfig,
) error {
	ticker := time.NewTicker(cfg.TickTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := processEvents(ctx, manager, repo, producer, cfg.Limit); err != nil {
				logger.WithError(err).Error("failed to sent events")
			}
		}
	}
}

func processEvents(
	ctx context.Context,
	manager ports.TxManager,
	repo ports.OutboxRepository,
	producer ports.Producer,
	limit int64,
) error {
	return manager.Do(ctx, func(ctx context.Context) error {
		events, err := repo.GetEvents(ctx, limit)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}

		if err := producer.SendMessages(ctx, events); err != nil {
			return err
		}

		ids := make([]uuid.UUID, len(events))
		for i, v := range events {
			ids[i] = v.EventID
		}

		return repo.SentEvent(ctx, ids)
	})
}
