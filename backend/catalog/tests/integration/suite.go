//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresImage    = "postgres:16-alpine"
	postgresUser     = "postgres"
	postgresPassword = "postgres"
	postgresDb       = "postgres"
	migrations       = "../../../../migrator/migrations/catalog"
)

type Suite struct {
	suite.Suite
	container *postgres.PostgresContainer
	catalog   *catalog.Repository
	outbox    *outbox.Repository
	pool      *pgxpool.Pool
	ctx       context.Context
}

func (s *Suite) SetupSuite() {
	s.ctx = context.Background()
	container, err := postgres.Run(
		s.ctx,
		postgresImage,
		postgres.WithDatabase(postgresDb),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPassword),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(s.T(), err)

	testcontainers.CleanupContainer(s.T(), container)

	s.container = container

	dsn, err := container.ConnectionString(s.ctx, "sslmode=disable")
	require.NoError(s.T(), err)

	pool := getPool(s.ctx, dsn)
	s.pool = pool
	s.catalog = catalog.NewRepository(pool, pool)
	s.outbox = outbox.NewRepository(pool, pool)

	require.NoError(s.T(), runMigrations(s.ctx, pool))
}

func (s *Suite) TearDownTest() {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `TRUNCATE TABLE catalog RESTART IDENTITY CASCADE`)
	s.Require().NoError(err, "failed to truncate catalog after test")
}

func (s *Suite) TearDownSuite() {
	_, err := s.pool.Exec(context.Background(), `TRUNCATE catalog RESTART IDENTITY CASCADE`)
	s.Require().NoError(err, "failed to truncate catalog after all tests")

	s.pool.Close()
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	std := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = std.Close()
	}()

	return goose.UpContext(ctx, std, migrations)
}

func (s *Suite) loadFixtures(path string) error {
	raw, err := os.ReadFile("fixtures/" + path)
	if err != nil {
		return fmt.Errorf("error while loading fixtures: %w", err)
	}

	_, err = s.pool.Exec(s.ctx, string(raw))
	if err != nil {
		return fmt.Errorf("error while executing fixtures: %w", err)
	}

	return nil
}
