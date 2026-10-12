package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
)

func testPoolConfig(dsn string) postgres.PoolConfig {
	return postgres.PoolConfig{
		DSN:               dsn,
		MaxConns:          4,
		MinConns:          1,
		MaxConnLifetime:   10 * time.Minute,
		MaxConnIdleTime:   2 * time.Minute,
		HealthCheckPeriod: 15 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}
}

func TestNewPool_AppliesConfig(t *testing.T) {
	pool := setupPool(t)
	want := testPoolConfig("")

	got := pool.Config()
	if got.MaxConns != want.MaxConns {
		t.Errorf("MaxConns = %d, want %d", got.MaxConns, want.MaxConns)
	}
	if got.MinConns != want.MinConns {
		t.Errorf("MinConns = %d, want %d", got.MinConns, want.MinConns)
	}
	if got.MaxConnLifetime != want.MaxConnLifetime {
		t.Errorf("MaxConnLifetime = %s, want %s", got.MaxConnLifetime, want.MaxConnLifetime)
	}
	if got.MaxConnIdleTime != want.MaxConnIdleTime {
		t.Errorf("MaxConnIdleTime = %s, want %s", got.MaxConnIdleTime, want.MaxConnIdleTime)
	}
	if got.HealthCheckPeriod != want.HealthCheckPeriod {
		t.Errorf("HealthCheckPeriod = %s, want %s", got.HealthCheckPeriod, want.HealthCheckPeriod)
	}
	if got.ConnConfig.ConnectTimeout != want.ConnectTimeout {
		t.Errorf("ConnectTimeout = %s, want %s", got.ConnConfig.ConnectTimeout, want.ConnectTimeout)
	}
	if maxConns := pool.Stat().MaxConns(); maxConns != want.MaxConns {
		t.Errorf("Stat().MaxConns() = %d, want %d", maxConns, want.MaxConns)
	}
}

func TestNewPool_InvalidDSN(t *testing.T) {
	_, err := postgres.NewPool(context.Background(), testPoolConfig("not a dsn"))
	if err == nil {
		t.Fatal("expected error for malformed DSN")
	}
}

func TestNewPool_ConnectTimeout(t *testing.T) {
	cfg := testPoolConfig("postgres://postgres:postgres@203.0.113.1:5432/auth-db?sslmode=disable")
	cfg.ConnectTimeout = 300 * time.Millisecond

	start := time.Now()
	_, err := postgres.NewPool(context.Background(), cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for unreachable postgres")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("NewPool returned after %s, want about %s", elapsed, cfg.ConnectTimeout)
	}
}
