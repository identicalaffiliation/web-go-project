package box

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	catalogRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	outboxRepository "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/outbox"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/rest"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/s3"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
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

	catalogRepo := catalogRepository.NewRepository(masterPool, replicaPool)
	outboxRepo := outboxRepository.NewRepository(masterPool, replicaPool)

	service := catalog.NewService(
		outboxRepo,
		catalogRepo,
		logger,
		manager.Must(trm.NewDefaultFactory(masterPool)),
		minioClient,
	)

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
