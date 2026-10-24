//go:build unit

package domain

import (
	"testing"
	"uuid"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestToDomainFromKafkaMessage(t *testing.T) {
	t.Run("auth users.created payload", func(t *testing.T) {
		value := []byte(`{
			"event_id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d",
			"user_id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e",
			"email": "user@example.com",
			"role": "customer",
			"occurred_at": "2026-10-21T10:00:00Z"
		}`)

		event, err := ToDomainFromKafkaMessage(kafka.Message{Value: value})
		require.NoError(t, err)
		require.Equal(t, uuid.MustParse("0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d"), event.EventID)
		require.Equal(t, uuid.MustParse("0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e"), event.UserID)
	})

	cases := map[string]string{
		"invalid json":     `not json`,
		"empty value":      ``,
		"invalid user id":  `{"event_id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d", "user_id": "not-uuid"}`,
		"missing event id": `{"user_id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e"}`,
		"missing user id":  `{"event_id": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d"}`,
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			event, err := ToDomainFromKafkaMessage(kafka.Message{Value: []byte(value)})
			require.ErrorIs(t, err, ErrInvalidEvent)
			require.Nil(t, event)
		})
	}
}
