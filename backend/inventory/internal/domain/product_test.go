//go:build unit

package domain

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestToDomainFromKafkaMessage(t *testing.T) {
	t.Run("valid message", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		expected := Product{
			ID:          uuid.New(),
			Title:       "aa",
			Description: "bb",
			Price:       123,
			Quantity:    10,
			ImageKey:    "products/image.png",
			CreatedAt:   now,
			UpdatedAt:   now.Add(time.Minute),
		}

		value, err := json.Marshal(expected)
		require.NoError(t, err)

		p, err := ToDomainFromKafkaMessage(kafka.Message{Value: value})
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Equal(t, expected.ID, p.ID)
		require.Equal(t, expected.Title, p.Title)
		require.Equal(t, expected.Description, p.Description)
		require.Equal(t, expected.Price, p.Price)
		require.Equal(t, expected.Quantity, p.Quantity)
		require.Equal(t, expected.ImageKey, p.ImageKey)
		require.True(t, expected.CreatedAt.Equal(p.CreatedAt))
		require.True(t, expected.UpdatedAt.Equal(p.UpdatedAt))
	})

	t.Run("json field names", func(t *testing.T) {
		value := []byte(`{
			"id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d",
			"title": "aa",
			"description": "bb",
			"price": 100,
			"quantity": 5,
			"imageKey": "key",
			"createdAt": "2026-10-21T10:00:00Z",
			"updatedAt": "2026-10-21T11:00:00Z"
		}`)

		p, err := ToDomainFromKafkaMessage(kafka.Message{Value: value})
		require.NoError(t, err)
		require.Equal(t, uuid.MustParse("0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d"), p.ID)
		require.Equal(t, "aa", p.Title)
		require.Equal(t, "bb", p.Description)
		require.Equal(t, int64(100), p.Price)
		require.Equal(t, int16(5), p.Quantity)
		require.Equal(t, "key", p.ImageKey)
		require.Equal(t, time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC), p.CreatedAt)
		require.Equal(t, time.Date(2026, 10, 21, 11, 0, 0, 0, time.UTC), p.UpdatedAt)
	})

	t.Run("invalid json", func(t *testing.T) {
		p, err := ToDomainFromKafkaMessage(kafka.Message{Value: []byte("not json")})
		require.Error(t, err)
		require.Nil(t, p)
	})

	t.Run("empty value", func(t *testing.T) {
		p, err := ToDomainFromKafkaMessage(kafka.Message{})
		require.Error(t, err)
		require.Nil(t, p)
	})

	t.Run("invalid id", func(t *testing.T) {
		p, err := ToDomainFromKafkaMessage(kafka.Message{Value: []byte(`{"id": "not-uuid"}`)})
		require.Error(t, err)
		require.Nil(t, p)
	})

	t.Run("quantity overflow", func(t *testing.T) {
		p, err := ToDomainFromKafkaMessage(kafka.Message{Value: []byte(`{"quantity": 40000}`)})
		require.Error(t, err)
		require.Nil(t, p)
	})
}
