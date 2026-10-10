//go:build integration

package integration

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	cacheredis "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
)

type CacheSuite struct {
	suite.Suite
	container *tcredis.RedisContainer
	client    *cacheredis.Client
	rdb       *goredis.Client
}

func (s *CacheSuite) SetupSuite() {
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:8-alpine")
	s.Require().NoError(err)
	testcontainers.CleanupContainer(s.T(), container)
	s.container = container

	host, err := container.Host(ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	s.Require().NoError(err)

	cfg := &config.RedisConfig{
		Host: host,
		Port: int(port.Num()),
	}

	s.client = cacheredis.NewClient(cfg)
	s.rdb = goredis.NewClient(&goredis.Options{
		Addr: host + ":" + port.Port(),
	})
}

func (s *CacheSuite) SetupTest() {
	s.Require().NoError(s.rdb.FlushDB(context.Background()).Err())
}

func (s *CacheSuite) TearDownSuite() {
	if s.rdb != nil {
		_ = s.rdb.Close()
	}
	if s.client != nil {
		_ = s.client.Close(context.Background())
	}
}
