package box

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
)

func providePostgres(
	ctx context.Context,
	timeout time.Duration,
	dsn string,
	cfg *config.PoolConfig,
) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(dsn, cfg)
	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(timeoutCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("new postgres pool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
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

func provideGrpcServer(service balancev1.PaymentsServiceServer) *grpc.Server {
	server := grpc.NewServer()
	balancev1.RegisterPaymentsServiceServer(server, service)
	return server
}

func provideListener(cfg *config.Config) (net.Listener, error) {
	conn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port))
	if err != nil {
		return nil, fmt.Errorf("listen tcp: %w", err)
	}

	return conn, nil
}
