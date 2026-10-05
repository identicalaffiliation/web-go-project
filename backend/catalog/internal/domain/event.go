package domain

import (
	"time"
	"uuid"
)

type Event struct {
	EventID   uuid.UUID
	Key       uuid.UUID
	Payload   []byte
	CreatedAt time.Time
	SentAt    time.Time
}
