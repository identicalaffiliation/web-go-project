package box

import (
	"context"
	"fmt"
	"net"
	"time"

	// Побочный импорт регистрирует сгенерированную спецификацию в swag.
	_ "github.com/identicalaffiliation/web-go-project/backend/orders/docs/orders"
	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/rest/controller"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/rest/middlewares"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// providePostgres - пул pgx с проверкой доступности базы. NewWithConfig сам к базе не ходит,
// поэтому без Ping сервис стартовал бы с недоступной базой и падал уже на первых запросах.
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

func provideGrpcServer(service ordersv1.OrdersServiceServer) *grpc.Server {
	server := grpc.NewServer()
	ordersv1.RegisterOrdersServiceServer(server, service)
	return server
}

func provideGrpcClient(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("new grpc client %s: %w", addr, err)
	}

	return conn, nil
}

func provideListener(cfg *config.Config) (net.Listener, error) {
	conn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port))
	if err != nil {
		return nil, fmt.Errorf("listen tcp: %w", err)
	}

	return conn, nil
}

func provideHttpServer(cfg *config.Config, service controller.Service) *echo.Echo {
	e := echo.New()
	e.Server.Addr = fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	e.Server.ReadTimeout = cfg.HTTP.ReadTimeout
	e.Server.WriteTimeout = cfg.HTTP.WriteTimeout
	e.Server.IdleTimeout = cfg.HTTP.IdleTimeout

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	e.POST(
		"/orders",
		controller.CreateOrder(service),
		middlewares.RoleMiddleware(middlewares.Customer),
	)

	return e
}
