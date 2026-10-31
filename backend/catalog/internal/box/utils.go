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

func getPostgresPool(
	ctx context.Context,
	duration time.Duration,
	dsn string,
	cfg *config.PoolConfig,
) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(dsn, cfg)
	if err != nil {
		return nil, fmt.Errorf("error GetPostgresPool: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeoutCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("error GetPostgresPool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error GetPostgresPool: %w", err)
	}

	return pool, nil
}

// poolConfig - pgxpool.Config из DSN с настройками пула из конфига.
// Нулевые поля конфига не трогают значения pgx по умолчанию (или pool_* из DSN).
func poolConfig(dsn string, cfg *config.PoolConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if poolCfg.MinConns > poolCfg.MaxConns {
		return nil, fmt.Errorf("postgres pool: min_conns (%d) > max_conns (%d)", poolCfg.MinConns, poolCfg.MaxConns)
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}
	// таймаут на каждое новое соединение, а не только на первое при старте
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	return poolCfg, nil
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
