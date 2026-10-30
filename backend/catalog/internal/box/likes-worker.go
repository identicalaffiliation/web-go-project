package box

import (
	"context"
	"sync"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/brokers/kafka"
	likes_outbox "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/likes-outbox"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/outbox"
)

func ProvideLikesWorker(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	masterPool, err := getPostgresPool(ctx, cfg.OperationDuration, cfg.MasterDSN)
	if err != nil {
		return err
	}

	defer masterPool.Close()

	replicaPool, err := getPostgresPool(ctx, cfg.OperationDuration, cfg.ReplicaDSN)
	if err != nil {
		return err
	}

	defer replicaPool.Close()

	repo := likes_outbox.NewRepository(masterPool, replicaPool)

	producer := kafka.NewProducer(&cfg.Broker, cfg.Outbox.Likes.Topic)

	defer func() {
		if err := producer.Close(); err != nil {
			logger.WithError(err).Error("failed to close producer conn")
		}
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	errChan := make(chan error, 1)
	go func() {
		if err := outbox.Run(ctx, repo, producer, logger, &cfg.Outbox.Likes, wg); err != nil {
			errChan <- err
		}
	}()

	wg.Wait()

	if val, ok := <-errChan; ok {
		close(errChan)
		return val
	}

	close(errChan)
	return nil
}
