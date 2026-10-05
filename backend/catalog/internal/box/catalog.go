package box

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	catalogRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	outboxRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/outbox"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/s3"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func SetupCatalogBox(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
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
	defer func() {
		if err := redisClient.Close(ctx); err != nil {
			slog.Error("failed to close redis client", "error", err)
		}
	}()

	kafkaproducer := kafka.NewProducer(&cfg.Broker)
	defer func() {
		if err := kafkaproducer.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	catalogRepo := catalogRepository.NewRepository(masterPool, replicaPool)
	outboxRepo := outboxRepository.NewRepository(masterPool, replicaPool)

	service := catalog.NewService(
		outboxRepo,
		catalogRepo,
		logger,
		manager.Must(trm.NewDefaultFactory(masterPool)),
		minioClient,
		redisClient,
	)

	var wg sync.WaitGroup
	notifyCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	wg.Add(1)
	go func() {
		if err := outbox.Run(notifyCtx, outboxRepo, kafkaproducer, logger, &cfg.Outbox, &wg); err != nil {
			logger.Debug("ctx signal")
		}
	}()

	server := rest.SetupServer(&cfg.HTTPConfig, service)

	notifyChan := make(chan os.Signal, 1)
	signal.Notify(notifyChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(notifyChan)

	errChan := make(chan error, 1)
	go func() {
		logger.Debug("start listening", zap.Int("port", cfg.HTTPConfig.Port))
		if err := server.Start(server.Server.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	select {
	case <-notifyChan:
		logger.Debug("server stopped by signal, graceful starts..")
	case serverError := <-errChan:
		logger.WithError(serverError).Error("failed to start server")
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.HTTPConfig.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("error ShutdownServer: %w", err)
	}

	wg.Wait()

	return nil
}

func getPostgresPool(ctx context.Context, duration time.Duration, dsn string) (*pgxpool.Pool, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	pool, err := pgxpool.New(timeoutCtx, dsn)
	if err != nil {
		return nil, fmt.Errorf("error GetMasterPool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		return nil, fmt.Errorf("error GetMasterPool: %w", err)
	}

	return pool, nil
}
