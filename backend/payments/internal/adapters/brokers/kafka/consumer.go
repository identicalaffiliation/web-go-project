package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/ports"
)

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type processer interface {
	ProcessTask(ctx context.Context, event *domain.UserCreated) error
}

type Consumer struct {
	reader        messageReader
	logger        ports.Logger
	taskProcesser processer
	retryBackoff  time.Duration
}

// NewConsumer - конструктор воркера, который читает из аутбокс топика регистрации пользователей.
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
		retryBackoff:  cfg.Kafka.RetryBackoff,
	}
}

// ReadMessage - цикл чтения. Битое сообщение коммитится и пропускается: повтор его не починит.
// Временная ошибка обработки ретраится на том же сообщении, пока не пройдет или не отменят контекст:
// offset коммитится только после успешной обработки, поэтому событие не теряется,
// а повтор безопасен - обработка идемпотентна по event_id.
func (c *Consumer) ReadMessage(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			c.logger.WithError(err).Error("failed to fetch message")
			continue
		}

		event, err := domain.ToDomainFromKafkaMessage(msg)
		if err != nil {
			c.logger.WithError(err).Error("failed to map kafka message to domain, skipping")
			c.commit(ctx, msg)
			continue
		}

		if !c.process(ctx, event) {
			return
		}

		c.commit(ctx, msg)
	}
}

// process - возвращает false, если контекст отменили до успешной обработки.
func (c *Consumer) process(ctx context.Context, event *domain.UserCreated) bool {
	for {
		err := c.taskProcesser.ProcessTask(ctx, event)
		if err == nil {
			return true
		}

		c.logger.WithError(err).Error("failed to process task, retrying",
			zap.String("event_id", event.EventID.String()))

		timer := time.NewTimer(c.retryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

func (c *Consumer) commit(ctx context.Context, msg kafka.Message) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		c.logger.WithError(err).Error("failed to commit message")
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
