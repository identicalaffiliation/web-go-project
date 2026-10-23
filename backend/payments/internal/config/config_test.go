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

	path := filepath.Join(t.TempDir(), "payments.yml")
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

kafka:
  brokers: [shop-kafka:9092]
  consume_topic: users.created
  group_id: payments-service
  queue_capacity: 50
  read_batch_timeout: 500ms
  workers: 1
  retry_backoff: 1s

temporal:
  host_port: shop-temporal:7233
  namespace: default
  task_queue: payments
  max_concurrent_activities: 50

initial_balance: 1000000
operation_duration: 30s
`)

		cfg, err := LoadConfig(path)
		require.NoError(t, err)

		require.Equal(t, "postgres://master", cfg.Postgres.MasterDSN)
		require.Equal(t, "postgres://replica", cfg.Postgres.ReplicaDSN)
		require.Equal(t, 50052, cfg.GRPC.Port)
		require.Equal(t, []string{"shop-kafka:9092"}, cfg.Kafka.Brokers)
		require.Equal(t, "users.created", cfg.Kafka.ConsumeTopic)
		require.Equal(t, "payments-service", cfg.Kafka.GroupID)
		require.Equal(t, 500*time.Millisecond, cfg.Kafka.ReadBatchTimeout)
		require.Equal(t, time.Second, cfg.Kafka.RetryBackoff)
		require.Equal(t, 1, cfg.Kafka.Workers)
		require.Equal(t, "payments", cfg.Temporal.TaskQueue)
		require.Equal(t, 50, cfg.Temporal.MaxConcurrentActivities)
		require.Equal(t, int64(1_000_000), cfg.InitialBalance)
		require.Equal(t, 30*time.Second, cfg.OperationDuration)
	})

	t.Run("project config is valid", func(t *testing.T) {
		cfg, err := LoadConfig("../../configs/payments.yml")
		require.NoError(t, err)
		require.Equal(t, "payments", cfg.Temporal.TaskQueue)
		require.Equal(t, "users.created", cfg.Kafka.ConsumeTopic)
		require.Equal(t, int64(1_000_000), cfg.InitialBalance)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml"))
		require.Error(t, err)
	})

	t.Run("broken yaml", func(t *testing.T) {
		_, err := LoadConfig(writeConfig(t, "grpc: [port"))
		require.Error(t, err)
	})
}
