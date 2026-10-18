package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// eventIDHeader — имя заголовка с идентификатором события (как в catalog).
const eventIDHeader = "event_id"

type Producer struct {
	writer *kafka.Writer
}

var _ ports.EventPublisher = (*Producer)(nil)

func NewProducer(brokers []string, writeTimeout time.Duration) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
			WriteTimeout: writeTimeout,
			Async:        false,
		},
	}
}

func (p *Producer) Publish(ctx context.Context, events []domain.OutboxEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, len(events))
	for i, e := range events {
		messages[i] = kafka.Message{
			Topic: e.Topic,
			Key:   []byte(e.Key.String()),
			Value: e.Payload,
			Headers: []kafka.Header{
				{Key: eventIDHeader, Value: []byte(e.ID.String())},
			},
		}
	}

	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return classifyWriteError(events, err)
	}
	return nil
}

func classifyWriteError(events []domain.OutboxEvent, err error) error {
	var writeErrs kafka.WriteErrors
	if !errors.As(err, &writeErrs) || len(writeErrs) != len(events) {
		return fmt.Errorf("write messages to kafka: %w", err)
	}

	partial := &ports.PartialPublishError{
		Rejected:    make(map[uuid.UUID]error),
		Undelivered: make(map[uuid.UUID]error),
	}
	for i, werr := range writeErrs {
		switch {
		case werr == nil:
		case isMessageRejected(werr):
			partial.Rejected[events[i].ID] = werr
		default:
			partial.Undelivered[events[i].ID] = werr
		}
	}
	return partial
}

func isMessageRejected(err error) bool {
	var kerr kafka.Error
	if !errors.As(err, &kerr) {
		return false
	}
	switch kerr {
	case kafka.MessageSizeTooLarge, kafka.RecordListTooLarge, kafka.InvalidMessage,
		kafka.InvalidMessageSize, kafka.InvalidTimestamp, kafka.InvalidRecord:
		return true
	default:
		return false
	}
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}
