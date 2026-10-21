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

	path := filepath.Join(t.TempDir(), "inventory.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		path := writeConfig(t, `
kafka:
    brokers: [broker-1:9092, broker-2:9092]
    consume_topic: stock.created
    group_id: inventory-service
    queue_capacity: 50
    read_batch_timeout: 500ms
    workers: 3
    tick_time: 300ms
`)

		cfg, err := LoadConfig(path)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Equal(t, []string{"broker-1:9092", "broker-2:9092"}, cfg.Kafka.Brokers)
		require.Equal(t, "stock.created", cfg.Kafka.ConsumeTopic)
		require.Equal(t, "inventory-service", cfg.Kafka.GroupID)
		require.Equal(t, 50, cfg.Kafka.QueueCapacity)
		require.Equal(t, 500*time.Millisecond, cfg.Kafka.ReadBatchTimeout)
		require.Equal(t, 3, cfg.Kafka.Workers)
		require.Equal(t, 300*time.Millisecond, cfg.Kafka.TickTime)
	})

	t.Run("project config", func(t *testing.T) {
		cfg, err := LoadConfig("../../configs/inventory.yml")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotEmpty(t, cfg.Kafka.Brokers)
		require.NotEmpty(t, cfg.Kafka.ConsumeTopic)
		require.NotEmpty(t, cfg.Kafka.GroupID)
		require.Positive(t, cfg.Kafka.Workers)
	})

	t.Run("file not found", func(t *testing.T) {
		cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml"))
		require.Error(t, err)
		require.Nil(t, cfg)
	})

	t.Run("empty path", func(t *testing.T) {
		cfg, err := LoadConfig("")
		require.Error(t, err)
		require.Nil(t, cfg)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		path := writeConfig(t, "kafka: [unclosed")

		cfg, err := LoadConfig(path)
		require.Error(t, err)
		require.Nil(t, cfg)
	})

	t.Run("invalid duration", func(t *testing.T) {
		path := writeConfig(t, `
kafka:
    tick_time: not-a-duration
`)

		cfg, err := LoadConfig(path)
		require.Error(t, err)
		require.Nil(t, cfg)
	})
}
