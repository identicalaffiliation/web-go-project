//go:build unit

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "orders.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		t.Setenv("MASTER_DSN", "postgres://master")
		t.Setenv("REPLICA_DSN", "postgres://replica")

		path := writeConfig(t, `
grpc:
  host: 0.0.0.0
  port: 50052

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 5s
  write_timeout: 5s
  idle_timeout: 30s
  shutdown_timeout: 30s

temporal:
  host_port: shop-temporal:7233
  namespace: default
  task_queue: orders
  max_concurrent_activities: 50

catalog:
  addr: catalog-service:50051
`)

		cfg, err := LoadConfig(path)
		require.NoError(t, err)

		require.Equal(t, "postgres://master", cfg.Postgres.MasterDSN)
		require.Equal(t, "postgres://replica", cfg.Postgres.ReplicaDSN)
		require.Equal(t, 50052, cfg.GRPC.Port)
		require.Equal(t, 8080, cfg.HTTP.Port)
		require.Equal(t, 5*time.Second, cfg.HTTP.ReadTimeout)
		require.Equal(t, 30*time.Second, cfg.HTTP.ShutdownTimeout)
		require.Equal(t, "shop-temporal:7233", cfg.Temporal.HostPort)
		require.Equal(t, "default", cfg.Temporal.Namespace)
		require.Equal(t, "orders", cfg.Temporal.TaskQueue)
		require.Equal(t, 50, cfg.Temporal.MaxConcurrentActivities)
		require.Equal(t, "catalog-service:50051", cfg.Catalog.Addr)
	})

	t.Run("project config is valid", func(t *testing.T) {
		cfg, err := LoadConfig("../../configs/orders.yml")
		require.NoError(t, err)
		require.Equal(t, "orders", cfg.Temporal.TaskQueue)
		require.NotEmpty(t, cfg.Catalog.Addr)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml"))
		require.Error(t, err)
	})

	t.Run("broken yaml", func(t *testing.T) {
		_, err := LoadConfig(writeConfig(t, "server: [port"))
		require.Error(t, err)
	})
}
