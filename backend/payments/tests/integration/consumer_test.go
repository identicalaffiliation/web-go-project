//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	brokerkafka "github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/brokers/kafka"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/account"
	"github.com/identicalaffiliation/web-go-project/backend/payments/pkg/logger"
)

// ConsumerSuite - консьюмер users.created с настоящими kafka и postgres.
type ConsumerSuite struct {
	suite.Suite

	db      Databases
	brokers []string
	service *account.Service

	topic   string
	groupID string
	writer  *kafka.Writer

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupSuite() {
	s.db = startPostgres(s.T())

	kc, err := tckafka.Run(s.T().Context(), kafkaImage)
	testcontainers.CleanupContainer(s.T(), kc)
	s.Require().NoError(err)

	s.brokers, err = kc.Brokers(s.T().Context())
	s.Require().NoError(err)

	log, err := logger.ProvideLogger()
	s.Require().NoError(err)

	repo := postgres.NewRepository(s.db.Master, s.db.Replica)
	s.service = account.NewService(repo, log, manager.Must(pgxv5.NewDefaultFactory(s.db.Master)), initialBalance)
}

func (s *ConsumerSuite) SetupTest() {
	s.topic = "users.created-" + uuid.New().String()
	s.groupID = "payments-" + uuid.New().String()

	s.createTopic()

	s.writer = &kafka.Writer{
		Addr:         kafka.TCP(s.brokers...),
		Topic:        s.topic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
	}
}

// createTopic - как в compose: одна партиция. Ждем лидера, иначе группа стартует без партиций.
func (s *ConsumerSuite) createTopic() {
	conn, err := kafka.DialContext(s.T().Context(), "tcp", s.brokers[0])
	s.Require().NoError(err)
	defer func() {
		_ = conn.Close()
	}()

	s.Require().NoError(conn.CreateTopics(kafka.TopicConfig{
		Topic:             s.topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}))

	s.Require().Eventually(func() bool {
		parts, err := conn.ReadPartitions(s.topic)
		return err == nil && len(parts) == 1 && parts[0].Leader.Host != ""
	}, waitTimeout, pollTick, "topic %q is not ready", s.topic)
}

func (s *ConsumerSuite) TearDownTest() {
	s.stopConsumer()
	s.Require().NoError(s.writer.Close())
	truncate(s.T(), s.db.Master)
}

func (s *ConsumerSuite) startConsumer() {
	cfg := &config.Config{Kafka: config.KafkaConfig{
		Brokers:          s.brokers,
		ConsumeTopic:     s.topic,
		GroupID:          s.groupID,
		QueueCapacity:    50,
		ReadBatchTimeout: 500 * time.Millisecond,
		RetryBackoff:     100 * time.Millisecond,
	}}

	log, err := logger.ProvideLogger()
	s.Require().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	consumer := brokerkafka.NewConsumer(cfg, log, s.service)
	s.wg.Go(func() {
		defer func() {
			_ = consumer.Close()
		}()

		consumer.ReadMessage(ctx)
	})
}

func (s *ConsumerSuite) stopConsumer() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	s.wg.Wait()
}

// payload в формате auth: backend/auth/internal/domain/outbox.go UserCreatedPayload
func userCreated(eventID, userID uuid.UUID) kafka.Message {
	value, _ := json.Marshal(map[string]any{
		"event_id":    eventID.String(),
		"user_id":     userID.String(),
		"email":       "user@example.com",
		"role":        "customer",
		"occurred_at": time.Now().UTC(),
	})

	return kafka.Message{Key: []byte(userID.String()), Value: value}
}

// send - пишет сообщения, повторяя запись, пока свежесозданный топик не станет доступен.
func (s *ConsumerSuite) send(msgs ...kafka.Message) {
	var lastErr error

	ok := s.Eventually(func() bool {
		ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
		defer cancel()

		lastErr = s.writer.WriteMessages(ctx, msgs...)

		return lastErr == nil
	}, waitTimeout, pollTick)

	s.Require().True(ok, "write messages: %v", lastErr)
}

func (s *ConsumerSuite) count(query string) int {
	var c int
	s.Require().NoError(s.db.Master.QueryRow(s.T().Context(), query).Scan(&c))

	return c
}

func (s *ConsumerSuite) waitCount(query string, want int) {
	ok := s.Eventually(func() bool {
		return s.count(query) == want
	}, waitTimeout, pollTick)

	s.Require().True(ok, "%s: got %d, want %d", query, s.count(query), want)
}

func (s *ConsumerSuite) balance(userID uuid.UUID) int64 {
	var b int64
	s.Require().NoError(s.db.Master.QueryRow(s.T().Context(), `SELECT balance FROM accounts WHERE user_id = $1`, userID).Scan(&b))

	return b
}

func (s *ConsumerSuite) Test_UserCreated_OpensAccount() {
	userID := uuid.NewV7()
	s.startConsumer()

	s.send(userCreated(uuid.NewV7(), userID))

	s.waitCount(`SELECT COUNT(*) FROM accounts`, 1)
	s.Equal(int64(initialBalance), s.balance(userID))
}

// повторная доставка и битые сообщения: деньги начисляются один раз, битое не блокирует топик
func (s *ConsumerSuite) Test_DuplicatesAndPoisonMessages() {
	first, second := uuid.NewV7(), uuid.NewV7()
	eventID := uuid.NewV7()
	s.startConsumer()

	s.send(
		userCreated(eventID, first),
		kafka.Message{Value: []byte("not json")},
		userCreated(eventID, first),
		userCreated(uuid.NewV7(), first),
		userCreated(uuid.NewV7(), second),
	)

	s.waitCount(`SELECT COUNT(*) FROM processed_events`, 3)
	s.Equal(2, s.count(`SELECT COUNT(*) FROM accounts`))
	s.Equal(int64(initialBalance), s.balance(first))
	s.Equal(int64(initialBalance), s.balance(second))
}

// после перезапуска консьюмера (offset не закоммичен/событие пришло снова) повторного начисления нет
func (s *ConsumerSuite) Test_RedeliveryAfterRestart() {
	userID, eventID := uuid.NewV7(), uuid.NewV7()
	s.startConsumer()

	s.send(userCreated(eventID, userID))
	s.waitCount(`SELECT COUNT(*) FROM accounts`, 1)

	_, err := s.db.Master.Exec(s.T().Context(), `UPDATE accounts SET balance = 5 WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	s.stopConsumer()
	s.groupID = "payments-" + uuid.New().String() // новая группа перечитает топик с начала
	s.startConsumer()

	s.send(userCreated(uuid.NewV7(), uuid.NewV7()))
	s.waitCount(`SELECT COUNT(*) FROM processed_events`, 2)

	s.Equal(int64(5), s.balance(userID))
}

// пока база недоступна, событие ретраится и не теряется
func (s *ConsumerSuite) Test_RetryUntilDatabaseRecovers() {
	userID := uuid.NewV7()

	_, err := s.db.Master.Exec(s.T().Context(), `ALTER TABLE accounts RENAME TO accounts_tmp`)
	s.Require().NoError(err)

	s.startConsumer()
	s.send(userCreated(uuid.NewV7(), userID))

	// даем консьюмеру несколько раз упасть
	s.Never(func() bool {
		return s.count(`SELECT COUNT(*) FROM processed_events`) > 0
	}, 2*time.Second, pollTick)

	_, err = s.db.Master.Exec(s.T().Context(), `ALTER TABLE accounts_tmp RENAME TO accounts`)
	s.Require().NoError(err)

	s.waitCount(`SELECT COUNT(*) FROM accounts`, 1)
	s.Equal(int64(initialBalance), s.balance(userID))
}
