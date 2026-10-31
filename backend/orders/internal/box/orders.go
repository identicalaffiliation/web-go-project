package box

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/rpc"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/temporal"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
	"go.uber.org/zap"
)

func ProvideOrdersBox(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	masterPool, err := providePostgres(ctx, cfg.OperationDuration, cfg.Postgres.MasterDSN, &cfg.Postgres.Master)
	if err != nil {
		return err
	}

	defer masterPool.Close()

	replicaPool, err := providePostgres(ctx, cfg.OperationDuration, cfg.Postgres.ReplicaDSN, &cfg.Postgres.Replica)
	if err != nil {
		return err
	}

	defer replicaPool.Close()

	repository := postgres.NewRepository(masterPool, replicaPool)

	catalogConn, err := provideGrpcClient(cfg.Catalog.Addr)
	if err != nil {
		return err
	}

	defer func() {
		if err := catalogConn.Close(); err != nil {
			logger.WithError(err).Error("failed to close catalog grpc connection")
		}
	}()

	temporalWorker, err := temporal.NewWorker(&cfg.Temporal, temporal.NewActivities(repository))
	if err != nil {
		return err
	}

	defer temporalWorker.Close()

	service := orders.NewService(
		repository,
		logger,
		manager.Must(pgxv5.NewDefaultFactory(masterPool)),
		catalog.NewClient(catalogConn),
		temporalWorker,
	)

	e := provideHttpServer(cfg, service)

	notifyCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errChan := make(chan error, 3)

	grpcServer := provideGrpcServer(rpc.NewRPChandler(service))

	go func() {
		logger.Debug("start http server on", zap.String("addr", e.Server.Addr))
		if err := e.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	listener, err := provideListener(cfg)
	if err != nil {
		return err
	}

	go func() {
		logger.Debug("start grpc server on", zap.String("addr", listener.Addr().String()))
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
			errChan <- err
		}
	}()

	go func() {
		logger.Debug("start temporal worker", zap.String("task_queue", cfg.Temporal.TaskQueue))
		if err := temporalWorker.Run(notifyCtx.Done()); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-notifyCtx.Done():
		logger.Debug("application stopped by signal")
	case serverErr := <-errChan:
		logger.WithError(serverErr).Error("failed to listen server")
	}

	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		return err
	}

	return nil
}
