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

// LikePayload - payload события лайка товара.
// Action - "add" (лайк поставлен) или "remove" (лайк убран).
type LikePayload struct {
	ProductID uuid.UUID `json:"product_id"`
	Action    string    `json:"action"`
}
