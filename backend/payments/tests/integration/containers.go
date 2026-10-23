//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:16-alpine"
	kafkaImage    = "confluentinc/confluent-local:7.5.0"
	temporalImage = "temporalio/temporal:1.9.1"
	migrations    = "../../../../migrator/migrations/payments"

	initialBalance = 1_000_000

	waitTimeout = 30 * time.Second
	pollTick    = 200 * time.Millisecond
)

// Databases - master и "реплика" в одном контейнере: две отдельные базы с одной схемой.
// Настоящую WAL репликацию тут не поднимаем - проверяем, что чтение уходит в нужный пул.
type Databases struct {
	Master  *pgxpool.Pool
	Replica *pgxpool.Pool
}

func startPostgres(t *testing.T) Databases {
	t.Helper()

	pg, err := postgres.Run(
		t.Context(),
		postgresImage,
		postgres.WithDatabase("payments"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, pg)
	require.NoError(t, err)

	masterDSN, err := pg.ConnectionString(t.Context(), "sslmode=disable")
	require.NoError(t, err)

	master := newPool(t, masterDSN)

	_, err = master.Exec(t.Context(), `CREATE DATABASE payments_replica`)
	require.NoError(t, err)

	host, err := pg.Host(t.Context())
	require.NoError(t, err)

	port, err := pg.MappedPort(t.Context(), "5432/tcp")
	require.NoError(t, err)

	replica := newPool(t, fmt.Sprintf(
		"postgres://postgres:postgres@%s:%s/payments_replica?sslmode=disable", host, port.Port(),
	))

	return Databases{Master: master, Replica: replica}
}

func newPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

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

func truncate(t *testing.T, pools ...*pgxpool.Pool) {
	t.Helper()

	for _, pool := range pools {
		_, err := pool.Exec(t.Context(), `TRUNCATE TABLE accounts, processed_events, payment_operations`)
		require.NoError(t, err)
	}
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
