package box

import (
	"context"
	"fmt"
	"net"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
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

func provideListener(cfg *config.Config) (net.Listener, error) {
	conn, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port))
	if err != nil {
		return nil, fmt.Errorf("listen tcp: %w", err)
	}

	return conn, nil
}
