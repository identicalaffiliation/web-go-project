package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutboxEvent — строка таблицы outbox: сообщение, которое нужно доставить в Kafka.
type OutboxEvent struct {
	ID        uuid.UUID // он же event_id для консьюмера
	Topic     string
	Key       uuid.UUID // ключ сообщения: события одного пользователя попадают в одну партицию
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
}

// UserCreatedPayload — тело события users.created.
// Совместим с backend/profile/internal/adapters/kafka/consumer.py: Profile читает
// из тела только user_id и event_id. Ключ сообщения и заголовок event_id нужны
// нам (порядок по партиции), Profile их не использует.
type UserCreatedPayload struct {
	EventID    uuid.UUID `json:"event_id"`
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	OccurredAt time.Time `json:"occurred_at"`
}

// NewUserCreatedEvent собирает событие регистрации. eventID передаётся снаружи,
// чтобы домен не зависел от генератора идентификаторов.
func NewUserCreatedEvent(user User, eventID uuid.UUID, topic string, now time.Time) (OutboxEvent, error) {
	payload, err := json.Marshal(UserCreatedPayload{
		EventID:    eventID,
		UserID:     user.ID,
		Email:      string(user.Email),
		Role:       string(user.Role),
		OccurredAt: now.UTC(),
	})
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("marshal users.created payload: %w", err)
	}

	return OutboxEvent{
		ID:        eventID,
		Topic:     topic,
		Key:       user.ID,
		Payload:   payload,
		CreatedAt: now,
	}, nil
}
