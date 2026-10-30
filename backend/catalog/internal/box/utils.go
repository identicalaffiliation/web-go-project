package box

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func getPostgresPool(ctx context.Context, duration time.Duration, dsn string) (*pgxpool.Pool, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	pool, err := pgxpool.New(timeoutCtx, dsn)
	if err != nil {
		return nil, fmt.Errorf("error GetPostgresPool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error GetPostgresPool: %w", err)
	}

	return pool, nil
}

// grpcListener - отдельный listener на каждый grpc-сервер (price, feedbacks).
func grpcListener(cfg *config.GRPCConfig) (net.Listener, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("error ListenGRPC on %d: %w", cfg.Port, err)
	}

	return listener, nil
}

func serveGRPC(server *grpc.Server, listener net.Listener, name string, logger ports.Logger) func() error {
	return func() error {
		logger.Debug("start grpc listening", zap.String("server", name), zap.String("addr", listener.Addr().String()))
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("%s grpc: %w", name, err)
		}

		return nil
	}
}

// stopGRPC - GracefulStop ждёт активные RPC; если не успели к дедлайну - рвём соединения.
func stopGRPC(ctx context.Context, server *grpc.Server) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		server.Stop()
		<-done
	}
}

func closeWithLog(logger ports.Logger, name string, closeFn func() error) {
	if err := closeFn(); err != nil {
		logger.WithError(err).Error("failed to close " + name)
	}
}
