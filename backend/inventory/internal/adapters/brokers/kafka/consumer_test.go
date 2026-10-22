//go:build unit

package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/adapters/postgres/stock"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
)

type testDeps struct {
	reader    *mocks.MockMessageReader
	logger    *mocks.MockLogger
	processer *mocks.MockProcesser
}

func newTestConsumer(t *testing.T) (*Consumer, testDeps) {
	t.Helper()

	deps := testDeps{
		reader:    mocks.NewMockMessageReader(t),
		logger:    mocks.NewMockLogger(t),
		processer: mocks.NewMockProcesser(t),
	}
	deps.logger.EXPECT().WithError(mock.Anything).Return(deps.logger).Maybe()

	return &Consumer{reader: deps.reader, logger: deps.logger, taskProcesser: deps.processer}, deps
}

// expectFetches - отдает сообщения по очереди, после последнего отменяет контекст и останавливает цикл чтения.
func expectFetches(ctx context.Context, cancel context.CancelFunc, reader *mocks.MockMessageReader, results ...func() (kafka.Message, error)) {
	for _, res := range results {
		reader.EXPECT().FetchMessage(mock.Anything).RunAndReturn(func(context.Context) (kafka.Message, error) {
			return res()
		}).Once()
	}

	reader.EXPECT().FetchMessage(mock.Anything).RunAndReturn(func(context.Context) (kafka.Message, error) {
		cancel()
		return kafka.Message{}, ctx.Err()
	}).Once()
}

func message(msg kafka.Message) func() (kafka.Message, error) {
	return func() (kafka.Message, error) { return msg, nil }
}

func fetchError(err error) func() (kafka.Message, error) {
	return func() (kafka.Message, error) { return kafka.Message{}, err }
}

func runReadMessage(t *testing.T, c *Consumer, ctx context.Context) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		c.ReadMessage(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadMessage did not return")
	}
}

func validMessage(offset int64) kafka.Message {
	return kafka.Message{
		Offset: offset,
		Value:  []byte(`{"id":"0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d","title":"aa","price":100,"quantity":5}`),
	}
}

func TestNewConsumer(t *testing.T) {
	cfg := &config.Config{Kafka: config.KafkaConfig{
		Brokers:      []string{"localhost:9092"},
		ConsumeTopic: "stock.created",
		GroupID:      "inventory-service",
	}}
	logger := mocks.NewMockLogger(t)
	processer := mocks.NewMockProcesser(t)

	c := NewConsumer(cfg, logger, processer)
	require.NotNil(t, c)
	require.NotNil(t, c.reader)
	require.Same(t, logger, c.logger)
	require.Same(t, processer, c.taskProcesser)
	require.NoError(t, c.Close())
}

func TestConsumer_ReadMessage(t *testing.T) {
	t.Run("processed messages are committed", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		msg1, msg2 := validMessage(1), validMessage(2)

		expectFetches(ctx, cancel, deps.reader, message(msg1), message(msg2))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Twice()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg1}).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg2}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("fetch error is logged and reading continues", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		msg := validMessage(1)

		expectFetches(ctx, cancel, deps.reader, fetchError(errors.New("broker unavailable")), message(msg))
		deps.logger.EXPECT().Error("failed to fetch message", mock.Anything).Once()
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("invalid message is committed and skipped", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		msg := kafka.Message{Offset: 1, Value: []byte("not json")}

		expectFetches(ctx, cancel, deps.reader, message(msg))
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()
		deps.logger.EXPECT().Error("failed to map kafka message to domain", mock.Anything).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("non retryable process error is committed", func(t *testing.T) {
		for _, processErr := range []error{stock.ErrAlreadyExists, stock.ErrInvalidData} {
			c, deps := newTestConsumer(t)
			ctx, cancel := context.WithCancel(context.Background())
			msg := validMessage(1)

			expectFetches(ctx, cancel, deps.reader, message(msg))
			deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(processErr).Once()
			deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()
			deps.logger.EXPECT().Error("failed to process task", mock.Anything).Once()

			runReadMessage(t, c, ctx)
		}
	})

	t.Run("retryable process error is not committed", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())

		expectFetches(ctx, cancel, deps.reader, message(validMessage(1)))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		deps.logger.EXPECT().Error("failed to process task", mock.Anything).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("commit error is logged", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		msg := validMessage(1)

		expectFetches(ctx, cancel, deps.reader, message(msg))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(errors.New("commit failed")).Once()
		deps.logger.EXPECT().Error("failed to commit message", mock.Anything).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("context canceled from fetch stops reading without error log", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())

		expectFetches(ctx, cancel, deps.reader)

		runReadMessage(t, c, ctx)
	})

	t.Run("already canceled context returns immediately", func(t *testing.T) {
		c, _ := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		runReadMessage(t, c, ctx)
	})
}

func TestConsumer_Close(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		deps.reader.EXPECT().Close().Return(nil).Once()

		require.NoError(t, c.Close())
	})

	t.Run("error", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		closeErr := errors.New("close failed")
		deps.reader.EXPECT().Close().Return(closeErr).Once()

		require.ErrorIs(t, c.Close(), closeErr)
	})
}
