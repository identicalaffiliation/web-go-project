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

	"github.com/identicalaffiliation/web-go-project/backend/payments/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
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
	deps.logger.EXPECT().Error(mock.Anything).Maybe()
	deps.logger.EXPECT().Error(mock.Anything, mock.Anything).Maybe()

	return &Consumer{
		reader:        deps.reader,
		logger:        deps.logger,
		taskProcesser: deps.processer,
		retryBackoff:  time.Millisecond,
	}, deps
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
		Value:  []byte(`{"event_id":"0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d","user_id":"0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e"}`),
	}
}

func TestConsumer_ReadMessage(t *testing.T) {
	t.Run("processes and commits", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		msg := validMessage(1)
		expectFetches(ctx, cancel, deps.reader, message(msg))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.MatchedBy(func(e *domain.UserCreated) bool {
			return e.UserID.String() == "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e"
		})).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("invalid message is committed and skipped", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		msg := kafka.Message{Offset: 1, Value: []byte("not json")}
		expectFetches(ctx, cancel, deps.reader, message(msg))
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()

		runReadMessage(t, c, ctx)
		deps.processer.AssertNotCalled(t, "ProcessTask", mock.Anything, mock.Anything)
	})

	t.Run("process error is retried on the same message before commit", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		msg := validMessage(1)
		expectFetches(ctx, cancel, deps.reader, message(msg))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(errors.New("db down")).Twice()
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("cancel during retry stops without commit", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		c.retryBackoff = time.Hour
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		deps.reader.EXPECT().FetchMessage(mock.Anything).Return(validMessage(1), nil).Once()
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).
			RunAndReturn(func(context.Context, *domain.UserCreated) error {
				cancel()
				return errors.New("db down")
			}).Once()

		runReadMessage(t, c, ctx)
		deps.reader.AssertNotCalled(t, "CommitMessages", mock.Anything, mock.Anything)
	})

	t.Run("fetch error continues reading", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		msg := validMessage(2)
		expectFetches(ctx, cancel, deps.reader, fetchError(errors.New("broker unavailable")), message(msg))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{msg}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})

	t.Run("commit error does not stop reading", func(t *testing.T) {
		c, deps := newTestConsumer(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		first, second := validMessage(1), validMessage(2)
		expectFetches(ctx, cancel, deps.reader, message(first), message(second))
		deps.processer.EXPECT().ProcessTask(mock.Anything, mock.Anything).Return(nil).Twice()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{first}).Return(errors.New("commit failed")).Once()
		deps.reader.EXPECT().CommitMessages(mock.Anything, []kafka.Message{second}).Return(nil).Once()

		runReadMessage(t, c, ctx)
	})
}

func TestConsumer_Close(t *testing.T) {
	c, deps := newTestConsumer(t)
	deps.reader.EXPECT().Close().Return(nil).Once()

	require.NoError(t, c.Close())
}
