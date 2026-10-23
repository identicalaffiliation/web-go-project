package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/segmentio/kafka-go"
)

var ErrInvalidEvent = errors.New("invalid user created event")

// UserCreated - событие users.created из outbox auth, из тела нужны только event_id и user_id.
// event_id - ключ идемпотентности консьюмера.
type UserCreated struct {
	EventID uuid.UUID `json:"event_id"`
	UserID  uuid.UUID `json:"user_id"`
}

func ToDomainFromKafkaMessage(msg kafka.Message) (*UserCreated, error) {
	event := new(UserCreated)
	if err := json.Unmarshal(msg.Value, event); err != nil {
		return nil, fmt.Errorf("%w: unmarshal kafka message: %w", ErrInvalidEvent, err)
	}

	if event.EventID == uuid.Nil() || event.UserID == uuid.Nil() {
		return nil, ErrInvalidEvent
	}

	return event, nil
}
