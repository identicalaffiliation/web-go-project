package box

import (
	"context"
	"errors"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/brokers/kafka"
	likes_outbox "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/likes-outbox"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/outbox"
)

func ProvideLikesWorker(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	masterPool, err := getPostgresPool(ctx, cfg.OperationDuration, cfg.MasterDSN, &cfg.Master)
	if err != nil {
		return err
	}

	defer masterPool.Close()

	replicaPool, err := getPostgresPool(ctx, cfg.OperationDuration, cfg.ReplicaDSN, &cfg.Replica)
	if err != nil {
		return err
	}

	defer replicaPool.Close()

	repo := likes_outbox.NewRepository(masterPool, replicaPool)

	producer := kafka.NewProducer(&cfg.Broker, cfg.Outbox.Likes.Topic)

	defer closeWithLog(logger, "likes producer", producer.Close)

	if err := outbox.Run(ctx, manager.Must(trm.NewDefaultFactory(masterPool)), repo, producer, logger, &cfg.Outbox.Likes); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
