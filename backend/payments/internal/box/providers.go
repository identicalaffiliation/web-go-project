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

func providePostgres(ctx context.Context, timeout time.Duration, dsn string) (*pgxpool.Pool, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pool, err := pgxpool.New(timeoutCtx, dsn)
	if err != nil {
		return nil, fmt.Errorf("new postgres pool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
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
