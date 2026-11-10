//go:build integration

package integration

import (
	"context"
	"testing"
	"time"
	"uuid"

	cacheredis "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

func (s *CacheSuite) Test_Cache_SetGet() {
	t := s.T()
	ctx := context.Background()

	expected := &domain.Product{
		ID:          uuid.New(),
		Title:       "some title",
		Description: "some description",
		Price:       120000,
		ImageKey:    "some key",
	}

	key := "product:" + expected.ID.String()
	s.Require().NoError(s.client.Set(ctx, key, expected))

	actual, err := s.client.Get(ctx, key)
	s.Require().NoError(err)
	s.Require().NotNil(actual)

	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.Title, actual.Title)
	require.Equal(t, expected.Description, actual.Description)
	require.Equal(t, expected.Price, actual.Price)
	require.Equal(t, expected.ImageKey, actual.ImageKey)
}

func (s *CacheSuite) Test_Cache_Get_CacheMiss() {
	ctx := context.Background()

	actual, err := s.client.Get(ctx, "nonexistent:"+uuid.New().String())
	s.Require().Error(err)
	s.Require().ErrorIs(err, cacheredis.ErrMiss)
	s.Require().Nil(actual)
}

func (s *CacheSuite) Test_Cache_Set_Overwrite() {
	ctx := context.Background()
	key := "product:overwrite"

	first := &domain.Product{
		ID:    uuid.New(),
		Title: "first",
		Price: 100,
	}
	s.Require().NoError(s.client.Set(ctx, key, first))

	second := &domain.Product{
		ID:    first.ID,
		Title: "second",
		Price: 200,
	}
	s.Require().NoError(s.client.Set(ctx, key, second))

	actual, err := s.client.Get(ctx, key)
	s.Require().NoError(err)
	s.Require().Equal("second", actual.Title)
	s.Require().Equal(int64(200), actual.Price)
}

func (s *CacheSuite) Test_Cache_Get_AfterFlush() {
	ctx := context.Background()

	expected := &domain.Product{
		ID:    uuid.New(),
		Title: "flushed",
		Price: 50,
	}
	key := "product:" + expected.ID.String()

	s.Require().NoError(s.client.Set(ctx, key, expected))
	s.Require().NoError(s.rdb.FlushDB(ctx).Err())

	actual, err := s.client.Get(ctx, key)
	s.Require().Error(err)
	s.Require().ErrorIs(err, cacheredis.ErrMiss)
	s.Require().Nil(actual)
}

func (s *CacheSuite) Test_Cache_Delete() {
	ctx := context.Background()

	expected := &domain.Product{ID: uuid.New(), Title: "deleted", Price: 50}
	key := "product:" + expected.ID.String()

	s.Require().NoError(s.client.Set(ctx, key, expected))
	s.Require().NoError(s.client.Delete(ctx, key))

	actual, err := s.client.Get(ctx, key)
	s.Require().ErrorIs(err, cacheredis.ErrMiss)
	s.Require().Nil(actual)

	s.Require().NoError(s.client.Delete(ctx, key), "missing key is not an error")
}

func (s *CacheSuite) Test_Cache_Set_TTLFromConfig() {
	ctx := context.Background()

	cfg := *s.cfg
	cfg.TTL = time.Minute
	client := cacheredis.NewClient(&cfg)
	defer func() { _ = client.Close(ctx) }()

	key := "product:" + uuid.New().String()
	s.Require().NoError(client.Set(ctx, key, &domain.Product{ID: uuid.New(), Title: "ttl", Price: 1}))

	ttl, err := s.rdb.TTL(ctx, key).Result()
	s.Require().NoError(err)
	s.Require().Greater(ttl, time.Duration(0))
	s.Require().LessOrEqual(ttl, time.Minute)
}
