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
// UserID - кто поставил/убрал лайк (по нему profile ведёт избранное пользователя).
// Action - "add" (лайк поставлен) или "remove" (лайк убран).
type LikePayload struct {
	UserID    uuid.UUID `json:"user_id"`
	ProductID uuid.UUID `json:"product_id"`
	Action    string    `json:"action"`
}
