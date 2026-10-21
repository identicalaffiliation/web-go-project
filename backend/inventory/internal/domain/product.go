package domain

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"
)

type Product struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	Quantity    int16     `json:"quantity"`
	ImageKey    string    `json:"imageKey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ToDomainFromKafkaMessage(msg kafka.Message) (*Product, error) {
	value := new(Product)
	if err := json.Unmarshal(msg.Value, value); err != nil {
		return nil, fmt.Errorf("unmarshal kafka message: %w", err)
	}

	return value, nil
}
