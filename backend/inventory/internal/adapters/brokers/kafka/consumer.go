package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
	"github.com/segmentio/kafka-go"
)

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader   messageReader
	tickTime time.Duration
	logger   ports.Logger
}

// NewConsumer - конструктор создания воркера, который будет читать из аутбокс топика создания стока.
// Не задаем партиции, потому что работаем в рамках одной группы и воркеры будут параллелить партиции.
func NewConsumer(cfg *config.Config, logger ports.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:          cfg.Kafka.Brokers,
			Topic:            cfg.Kafka.ConsumeTopic,
			GroupID:          cfg.Kafka.GroupID,
			QueueCapacity:    cfg.Kafka.QueueCapacity,
			ReadBatchTimeout: cfg.Kafka.ReadBatchTimeout,
		}),
		tickTime: cfg.Kafka.TickTime,
		logger:   logger,
	}
}

func (c *Consumer) ReadMessage(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				c.logger.WithError(err).Error("failed to fetch message")
				continue
			}

			product, err := domain.ToDomainFromKafkaMessage(msg)
			if err != nil {
				c.logger.WithError(err).Error("failed to map kafka message to domain")
				continue
			}

			fmt.Println(product)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.WithError(err).Error("failed to commit message")
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
