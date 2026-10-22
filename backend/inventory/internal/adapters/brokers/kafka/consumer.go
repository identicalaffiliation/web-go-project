package kafka

import (
	"context"
	"errors"

	"github.com/segmentio/kafka-go"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/postgres/stock"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
)

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type processer interface {
	ProcessTask(ctx context.Context, product *domain.Product) error
}

type Consumer struct {
	reader        messageReader
	logger        ports.Logger
	taskProcesser processer
}

// NewConsumer - конструктор создания воркера, который будет читать из аутбокс топика создания стока.
// Не задаем партиции, потому что работаем в рамках одной группы и воркеры будут параллелить партиции.
func NewConsumer(cfg *config.Config, logger ports.Logger, taskProcesser processer) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:          cfg.Kafka.Brokers,
			Topic:            cfg.Kafka.ConsumeTopic,
			GroupID:          cfg.Kafka.GroupID,
			QueueCapacity:    cfg.Kafka.QueueCapacity,
			ReadBatchTimeout: cfg.Kafka.ReadBatchTimeout,
		}),
		logger:        logger,
		taskProcesser: taskProcesser,
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
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					c.logger.WithError(err).Error("failed to commit message")
					continue
				}

				c.logger.WithError(err).Error("failed to map kafka message to domain")
				continue
			}

			if err := c.taskProcesser.ProcessTask(ctx, product); err != nil {
				if errors.Is(err, stock.ErrAlreadyExists) || errors.Is(err, stock.ErrInvalidData) {
					if err := c.reader.CommitMessages(ctx, msg); err != nil {
						c.logger.WithError(err).Error("failed to commit message")
						continue
					}
				}

				c.logger.WithError(err).Error("failed to process task")
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.WithError(err).Error("failed to commit message")
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
