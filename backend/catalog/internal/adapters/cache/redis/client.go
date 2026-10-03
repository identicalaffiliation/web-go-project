package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/redis/go-redis/v9"
)

var (
	ErrMiss = errors.New("cache miss")
)

type Client struct {
	client *redis.Client
	ttl    time.Duration
}

func NewClient(cfg *config.RedisConfig) *Client {
	pool := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       0,
	})

	return &Client{client: pool}
}

func (c *Client) Set(ctx context.Context, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal product: %w", err)
	}

	return c.client.Set(ctx, key, b, c.ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string) (*domain.Product, error) {
	b, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrMiss
		}

		return nil, fmt.Errorf("error GetProduct: %w", err)
	}

	var p domain.Product
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("unmarshal prodyuct: %w", err)
	}

	return &p, nil
}

func (c *Client) Close(ctx context.Context) error {
	return c.client.Close()
}
