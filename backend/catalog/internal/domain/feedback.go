package domain

import (
	"time"
	"uuid"
)

type Feedback struct {
	ID          uuid.UUID
	ProductID   uuid.UUID
	UserID      uuid.UUID
	Score       int8
	Description *string
	CreatedAt   time.Time
}
