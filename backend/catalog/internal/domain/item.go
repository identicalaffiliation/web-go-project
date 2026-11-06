package domain

import (
	"time"
	"uuid"
)

type Product struct {
	ID          uuid.UUID `json:"id"`
	BusinessID  uuid.UUID `json:"businessId"` // user_id бизнеса, выложившего товар
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	Quantity    int16     `json:"quantity"`
	ImageKey    string    `json:"imageKey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	Rating         float64 `json:"rating"`
	FeedbacksCount int64   `json:"feedbacksCount"`
}
