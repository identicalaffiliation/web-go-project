//go:build integration

package integration

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pricev1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/price/v1"
)

const (
	postgresImage = "postgres:16-alpine"
	temporalImage = "temporalio/temporal:1.9.1"
	migrations    = "../../../../migrator/migrations/orders"

	waitTimeout = 30 * time.Second
	pollTick    = 200 * time.Millisecond
)

func startPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pg, err := postgres.Run(
		t.Context(),
		postgresImage,
		postgres.WithDatabase("orders"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, pg)
	require.NoError(t, err)

	dsn, err := pg.ConnectionString(t.Context(), "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(t.Context(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, runMigrations(t.Context(), pool))

	return pool
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = db.Close()
	}()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.UpContext(ctx, db, migrations)
}

func startTemporal(t *testing.T) string {
	t.Helper()

	tc, err := testcontainers.Run(
		t.Context(),
		temporalImage,
		testcontainers.WithCmd("server", "start-dev", "--ip", "0.0.0.0", "--headless"),
		testcontainers.WithExposedPorts("7233/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("7233/tcp").WithStartupTimeout(time.Minute)),
	)
	testcontainers.CleanupContainer(t, tc)
	require.NoError(t, err)

	hostPort, err := tc.PortEndpoint(t.Context(), "7233/tcp", "")
	require.NoError(t, err)

	return hostPort
}

type fakeCatalog struct {
	pricev1.UnimplementedPriceServiceServer

	mu     sync.Mutex
	prices map[string]int64
}

func (c *fakeCatalog) GetPriceByProductId(_ context.Context, req *pricev1.PriceRequest) (*pricev1.PriceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	price, ok := c.prices[req.GetProductId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}

	return &pricev1.PriceResponse{ProductId: req.GetProductId(), Price: price}, nil
}

func (c *fakeCatalog) setPrice(productID uuid.UUID, price int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.prices[productID.String()] = price
}

// startFakeCatalog - запускает fakeCatalog на свободном порту и возвращает коннект к нему.
func startFakeCatalog(t *testing.T) (*fakeCatalog, *grpc.ClientConn) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	catalog := &fakeCatalog{prices: make(map[string]int64)}
	server := grpc.NewServer()
	pricev1.RegisterPriceServiceServer(server, catalog)

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return catalog, conn
}
