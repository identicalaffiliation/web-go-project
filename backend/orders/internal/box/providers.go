package box

import (
	"context"
	"fmt"
	"net"

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

func providePostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("new postgres pool: %w", err)
	}

	return pool, nil
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
