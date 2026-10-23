package box

import (
	"context"
	"errors"
	"net"
	"os/signal"
	"sync"
	"syscall"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/rpc"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/temporal"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/account"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/balance"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/workflow"
)

func ProvidePaymentsBox(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	notifyCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	masterPool, err := providePostgres(notifyCtx, cfg.OperationDuration, cfg.Postgres.MasterDSN)
	if err != nil {
		return err
	}

	defer masterPool.Close()

	replicaPool, err := providePostgres(notifyCtx, cfg.OperationDuration, cfg.Postgres.ReplicaDSN)
	if err != nil {
		return err
	}

	defer replicaPool.Close()

	repository := postgres.NewRepository(masterPool, replicaPool)
	txManager := manager.Must(pgxv5.NewDefaultFactory(masterPool))

	accountService := account.NewService(repository, logger, txManager, cfg.InitialBalance)
	workflowService := workflow.NewService(repository, logger, txManager)
	balanceService := balance.NewService(repository, logger)

	temporalWorker, err := temporal.NewWorker(&cfg.Temporal, temporal.NewActivities(workflowService))
	if err != nil {
		return err
	}

	defer temporalWorker.Close()

	listener, err := provideListener(cfg)
	if err != nil {
		return err
	}

	grpcServer := provideGrpcServer(rpc.NewRPChandler(balanceService))
	errChan := make(chan error, 2)

	var wg sync.WaitGroup

	for range cfg.Kafka.Workers {
		wg.Go(func() {
			consumer := kafka.NewConsumer(cfg, logger, accountService)
			defer func() {
				if err := consumer.Close(); err != nil {
					logger.WithError(err).Error("failed to close consumer")
				}
			}()

			consumer.ReadMessage(notifyCtx)
		})
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

	var runErr error
	select {
	case <-notifyCtx.Done():
		logger.Debug("application stopped by signal")
	case runErr = <-errChan:
		logger.WithError(runErr).Error("failed to run application")
	}

	// останавливаем консьюмеров и воркер, если выход по ошибке
	stop()
	grpcServer.GracefulStop()
	wg.Wait()

	return runErr
}
