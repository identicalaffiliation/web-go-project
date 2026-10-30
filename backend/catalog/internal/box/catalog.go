package box

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	catalogRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	likes_outbox "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/likes-outbox"
	outboxRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/outbox"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rpc/price"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/s3"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/outbox"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// SetupCatalogBox - поднимает REST, grpc (price, feedbacks) и stock outbox-воркер.
// Падение любого компонента или SIGINT/SIGTERM останавливает всё остальное.
func SetupCatalogBox(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	minioClient, err := s3.NewClient(&cfg.MinioConfig)
	if err != nil {
		return err
	}

	redisClient := redis.NewClient(&cfg.Cache)
	defer closeWithLog(logger, "redis client", func() error { return redisClient.Close(context.Background()) })

	stockProducer := kafka.NewProducer(&cfg.Broker, cfg.Outbox.Stock.Topic)
	defer closeWithLog(logger, "stock producer", stockProducer.Close)

	stockOutboxRepo := outboxRepository.NewRepository(masterPool, replicaPool)

	catalogService := catalog.NewService(
		stockOutboxRepo,
		catalogRepository.NewRepository(masterPool, replicaPool),
		likes_outbox.NewRepository(masterPool, replicaPool),
		logger,
		manager.Must(trm.NewDefaultFactory(masterPool)),
		minioClient,
		redisClient,
	)

	feedbackService, feedbacksGRPCServer := setupFeedbacks(masterPool, replicaPool, logger)
	priceGRPCServer := price.NewGRPCServer(catalogService)
	httpServer := rest.SetupServer(&cfg.HTTPConfig, catalogService, feedbackService)

	priceListener, err := grpcListener(&cfg.GRPCConfig)
	if err != nil {
		return err
	}

	feedbacksListener, err := grpcListener(&cfg.FeedbacksGRPC)
	if err != nil {
		closeWithLog(logger, "price listener", priceListener.Close)
		return err
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := outbox.Run(gCtx, stockOutboxRepo, stockProducer, logger, &cfg.Outbox.Stock); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stock outbox: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		logger.Debug("start http listening", zap.String("addr", httpServer.Server.Addr))
		if err := httpServer.Start(httpServer.Server.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http: %w", err)
		}

		return nil
	})

	g.Go(serveGRPC(priceGRPCServer, priceListener, "price", logger))
	g.Go(serveGRPC(feedbacksGRPCServer, feedbacksListener, "feedbacks", logger))

	// graceful: ждём сигнал или падение любого компонента, затем гасим серверы
	g.Go(func() error {
		<-gCtx.Done()
		logger.Debug("shutdown started", zap.NamedError("cause", context.Cause(gCtx)))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPConfig.ShutdownTimeout)
		defer cancel()

		stopGRPC(shutdownCtx, priceGRPCServer)
		stopGRPC(shutdownCtx, feedbacksGRPCServer)

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}

		return nil
	})

	return g.Wait()
}
