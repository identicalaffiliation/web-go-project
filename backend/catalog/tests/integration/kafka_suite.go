//go:build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"uuid"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	brokerkafka "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
)

const (
	kafkaImage = "confluentinc/confluent-local:7.5.0"
	readWait   = 15 * time.Second
)

type KafkaSuite struct {
	suite.Suite
	container *tckafka.KafkaContainer
	producer  *brokerkafka.Producer
	brokers   []string
	topic     string
}

func (s *KafkaSuite) SetupSuite() {
	ctx := context.Background()

	container, err := tckafka.Run(ctx, kafkaImage)
	s.Require().NoError(err)
	testcontainers.CleanupContainer(s.T(), container)
	s.container = container

	brokers, err := container.Brokers(ctx)
	s.Require().NoError(err)
	s.brokers = brokers
}

func (s *KafkaSuite) SetupTest() {
	ctx := context.Background()
	s.topic = "test-events-" + uuid.New().String()

	s.Require().NoError(s.createTopic(ctx))
	time.Sleep(8 * time.Second)

	cfg := &config.KafkaConfig{
		Brokers:      s.brokers,
		ProduceTopic: s.topic,
		BatchSize:    1,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 5 * time.Second,
	}
	s.producer = brokerkafka.NewProducer(cfg)
}

func (s *KafkaSuite) createTopic(ctx context.Context) error {
	conn, err := kafka.DialContext(ctx, "tcp", s.brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}

	if err := conn.CreateTopics(kafka.TopicConfig{
		Topic:             s.topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}); err != nil {
		_ = conn.Close()
		return fmt.Errorf("create topic: %w", err)
	}
	_ = conn.Close()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		c, err := kafka.DialContext(ctx, "tcp", s.brokers[0])
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}

		parts, err := c.ReadPartitions(s.topic)
		_ = c.Close()

		if err == nil && len(parts) > 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

	return fmt.Errorf("topic %q not available in metadata", s.topic)
}

func (s *KafkaSuite) TearDownTest() {
	if s.producer != nil {
		_ = s.producer.Close()
	}
}

func (s *KafkaSuite) newReader(groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  s.brokers,
		Topic:    s.topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
}
