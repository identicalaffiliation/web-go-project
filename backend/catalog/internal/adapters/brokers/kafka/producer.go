package kafka

import (
	"context"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/segmentio/kafka-go"
)

const (
	key = "event_id"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(cfg *config.KafkaConfig) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Topic:        cfg.ProduceTopic,
			MaxAttempts:  3,
			BatchSize:    cfg.BatchSize,
			BatchTimeout: cfg.BatchTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
	}
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) SendMessages(ctx context.Context, messages []*domain.Event) error {
	if err := p.writer.WriteMessages(ctx, p.formirateBatch(messages)...); err != nil {
		return fmt.Errorf("write messages: %w", err)
	}

	return nil
}

func (p *Producer) formirateBatch(messages []*domain.Event) []kafka.Message {
	events := make([]kafka.Message, len(messages))
	for i, v := range messages {
		events[i] = kafka.Message{
			Key:   []byte(v.Key.String()),
			Value: v.Payload,
			Headers: []kafka.Header{
				{
					Key:   key,
					Value: []byte(v.EventID.String()),
				},
			},
		}
	}

	return events
}
