package box

import (
	"context"
	"errors"
	"net"
	"os/signal"
	"syscall"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/rpc"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
	"go.uber.org/zap"
)

func ProvideOrdersBox(ctx context.Context, cfg *config.Config, logger ports.Logger) error {
	masterPool, err := providePostgres(ctx, cfg.Postgres.MasterDSN)
	if err != nil {
		return err
	}

	defer masterPool.Close()

	replicaPool, err := providePostgres(ctx, cfg.Postgres.ReplicaDSN)
	if err != nil {
		return err
	}

	defer replicaPool.Close()

	repository := postgres.NewRepository(masterPool, replicaPool)

	service := orders.NewService(
		repository,
		logger,
		manager.Must(pgxv5.NewDefaultFactory(masterPool)),
	)

	notifyCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errChan := make(chan error, 2)

	grpcServer := provideGrpcServer(rpc.NewRPChandler(service))
	listener, err := provideListener(cfg)
	if err != nil {
		return err
	}

	go func() {
		logger.Debug("start grpc server on", zap.String("addr", listener.Addr().String()))
		if err := grpcServer.Serve(listener); err != nil && errors.Is(err, net.ErrClosed) {
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
	return nil
}
