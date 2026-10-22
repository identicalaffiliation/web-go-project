//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	brokerkafka "github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/postgres/stock"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
	taskconsumer "github.com/identicalaffiliation/web-go-project/backend/inventory/internal/service/task-consumer"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/pkg/logger"
)

const (
	postgresImage = "postgres:16-alpine"
	kafkaImage    = "confluentinc/confluent-local:7.5.0"
	migrations    = "../../../../migrator/migrations/inventory"

	waitTimeout = 30 * time.Second
	pollTick    = 200 * time.Millisecond
)

// ConsumerSuite - консьюмер стока с настоящими kafka, postgres, процессором и репозиторием.
type ConsumerSuite struct {
	suite.Suite

	pool    *pgxpool.Pool
	brokers []string
	logger  ports.Logger

	topic   string
	groupID string
	writer  *kafka.Writer

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (s *ConsumerSuite) SetupSuite() {
	pg, err := postgres.Run(
		s.T().Context(),
		postgresImage,
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	testcontainers.CleanupContainer(s.T(), pg)

	dsn, err := pg.ConnectionString(s.T().Context(), "sslmode=disable")
	s.Require().NoError(err)

	s.pool, err = pgxpool.New(s.T().Context(), dsn)
	s.Require().NoError(err)
	s.Require().NoError(runMigrations(s.T().Context(), s.pool))

	kc, err := tckafka.Run(s.T().Context(), kafkaImage)
	s.Require().NoError(err)
	testcontainers.CleanupContainer(s.T(), kc)

	s.brokers, err = kc.Brokers(s.T().Context())
	s.Require().NoError(err)

	s.logger, err = logger.ProvideLogger()
	s.Require().NoError(err)
}

func (s *ConsumerSuite) TearDownSuite() {
	s.pool.Close()
}

func (s *ConsumerSuite) SetupTest() {
	s.topic = "stock.created-" + uuid.New().String()
	s.groupID = "inventory-" + uuid.New().String()

	s.writer = &kafka.Writer{
		Addr:         kafka.TCP(s.brokers...),
		Topic:        s.topic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
	}
}

func (s *ConsumerSuite) TearDownTest() {
	s.stopConsumers()
	s.Require().NoError(s.writer.Close())

	_, err := s.pool.Exec(s.T().Context(), `TRUNCATE TABLE stock, idempotency_keys`)
	s.Require().NoError(err)
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		_ = db.Close()
	}()

	return goose.UpContext(ctx, db, migrations)
}

// createTopic - создает топик и ждет, пока у всех партиций появится лидер.
func (s *ConsumerSuite) createTopic(partitions int) {
	conn, err := kafka.DialContext(s.T().Context(), "tcp", s.brokers[0])
	s.Require().NoError(err)
	defer func() {
		_ = conn.Close()
	}()

	s.Require().NoError(conn.CreateTopics(kafka.TopicConfig{
		Topic:             s.topic,
		NumPartitions:     partitions,
		ReplicationFactor: 1,
	}))

	s.Require().Eventually(func() bool {
		parts, err := conn.ReadPartitions(s.topic)
		if err != nil || len(parts) != partitions {
			return false
		}

		for _, p := range parts {
			if p.Leader.Host == "" {
				return false
			}
		}

		return true
	}, waitTimeout, pollTick, "topic %q is not ready", s.topic)
}

// startConsumers - запускает workers консьюмеров в одной группе, как в main.
func (s *ConsumerSuite) startConsumers(workers int) {
	cfg := &config.Config{Kafka: config.KafkaConfig{
		Brokers:          s.brokers,
		ConsumeTopic:     s.topic,
		GroupID:          s.groupID,
		QueueCapacity:    50,
		ReadBatchTimeout: 500 * time.Millisecond,
	}}
	processer := taskconsumer.NewTaskProcesser(stock.NewRepository(s.pool))

	ctx, cancel := context.WithCancel(s.T().Context())
	s.cancel = cancel

	for range workers {
		consumer := brokerkafka.NewConsumer(cfg, s.logger, processer)
		s.wg.Go(func() {
			defer func() {
				_ = consumer.Close()
			}()

			consumer.ReadMessage(ctx)
		})
	}
}

func (s *ConsumerSuite) stopConsumers() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	s.wg.Wait()
}

func (s *ConsumerSuite) sendProducts(products ...domain.Product) {
	msgs := make([]kafka.Message, 0, len(products))
	for _, p := range products {
		value, err := json.Marshal(p)
		s.Require().NoError(err)

		msgs = append(msgs, kafka.Message{Key: []byte(p.ID.String()), Value: value})
	}

	s.sendRaw(msgs...)
}

// sendRaw - пишет сообщения, повторяя запись, пока метаданные свежесозданного топика не дойдут до брокера.
func (s *ConsumerSuite) sendRaw(msgs ...kafka.Message) {
	var lastErr error

	ok := s.Eventually(func() bool {
		ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
		defer cancel()

		lastErr = s.writer.WriteMessages(ctx, msgs...)

		return lastErr == nil
	}, waitTimeout, pollTick)

	s.Require().True(ok, "write messages: %v", lastErr)
}

func newProduct(quantity int16) domain.Product {
	now := time.Now().UTC()

	return domain.Product{
		ID:        uuid.New(),
		Title:     "product",
		Price:     100,
		Quantity:  quantity,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// committedOffsets - закоммиченный группой offset по каждой партиции топика (-1, если коммита не было).
// Пока группа не создана, брокер отвечает ошибкой координатора - ее отдаем наверх, чтобы ожидание продолжилось.
func (s *ConsumerSuite) committedOffsets(partitions int) ([]int64, error) {
	ids := make([]int, partitions)
	for i := range ids {
		ids[i] = i
	}

	client := &kafka.Client{Addr: kafka.TCP(s.brokers...), Timeout: 5 * time.Second}
	resp, err := client.OffsetFetch(s.T().Context(), &kafka.OffsetFetchRequest{
		GroupID: s.groupID,
		Topics:  map[string][]int{s.topic: ids},
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	offsets := make([]int64, partitions)
	for _, p := range resp.Topics[s.topic] {
		if p.Error != nil {
			return nil, p.Error
		}

		offsets[p.Partition] = p.CommittedOffset
	}

	return offsets, nil
}

// waitCommitted - ждет, пока группа закоммитит offset на всех партициях. Сумма == число обработанных сообщений.
func (s *ConsumerSuite) waitCommitted(partitions int, total int64) {
	var (
		last    []int64
		lastErr error
	)

	ok := s.Eventually(func() bool {
		last, lastErr = s.committedOffsets(partitions)
		if lastErr != nil {
			return false
		}

		var sum int64
		for _, off := range last {
			if off > 0 {
				sum += off
			}
		}

		return sum == total
	}, waitTimeout, pollTick)

	s.Require().True(ok, "committed offsets: %v (err: %v), want total %d", last, lastErr, total)
}

func (s *ConsumerSuite) stockQuantity(productID uuid.UUID) (int16, bool) {
	var qty int16
	err := s.pool.QueryRow(s.T().Context(), `SELECT quantity FROM stock WHERE product_id = $1`, productID).Scan(&qty)
	if err != nil {
		return 0, false
	}

	return qty, true
}

func (s *ConsumerSuite) stockCount() int {
	var count int
	s.Require().NoError(s.pool.QueryRow(s.T().Context(), `SELECT COUNT(*) FROM stock`).Scan(&count))

	return count
}

func (s *ConsumerSuite) insertStock(productID uuid.UUID, quantity int16) {
	_, err := s.pool.Exec(s.T().Context(), `INSERT INTO stock (product_id, quantity) VALUES ($1, $2)`, productID, quantity)
	s.Require().NoError(err, fmt.Sprintf("insert stock %s", productID))
}
