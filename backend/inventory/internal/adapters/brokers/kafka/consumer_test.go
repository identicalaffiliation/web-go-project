//go:build unit

package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/ports"
)

type fetchResult struct {
	msg kafka.Message
	err error
}

type fakeReader struct {
	mu        sync.Mutex
	results   []fetchResult
	commitErr error
	committed []kafka.Message
	closeErr  error
	closed    bool
	cancel    context.CancelFunc
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.results) == 0 {
		r.cancel()
		return kafka.Message{}, ctx.Err()
	}

	res := r.results[0]
	r.results = r.results[1:]

	return res.msg, res.err
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, msgs...)

	return nil
}

func (r *fakeReader) Close() error {
	r.closed = true
	return r.closeErr
}

type fakeLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *fakeLogger) With(...zap.Field) ports.Logger { return l }
func (l *fakeLogger) WithError(error) ports.Logger   { return l }
func (l *fakeLogger) Debug(string, ...zap.Field)     {}
func (l *fakeLogger) Sync() error                    { return nil }

func (l *fakeLogger) Error(msg string, _ ...zap.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}

func newTestConsumer(results ...fetchResult) (*Consumer, *fakeReader, *fakeLogger, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &fakeReader{results: results, cancel: cancel}
	logger := &fakeLogger{}

	return &Consumer{reader: reader, logger: logger}, reader, logger, ctx
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
		TickTime:     300 * time.Millisecond,
	}}
	logger := &fakeLogger{}

	c := NewConsumer(cfg, logger)
	require.NotNil(t, c)
	require.NotNil(t, c.reader)
	require.Equal(t, cfg.Kafka.TickTime, c.tickTime)
	require.Same(t, logger, c.logger)
	require.NoError(t, c.Close())
}

func TestConsumer_ReadMessage(t *testing.T) {
	t.Run("valid messages are committed", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(
			fetchResult{msg: validMessage(1)},
			fetchResult{msg: validMessage(2)},
		)

		runReadMessage(t, c, ctx)

		require.Len(t, reader.committed, 2)
		require.Equal(t, int64(1), reader.committed[0].Offset)
		require.Equal(t, int64(2), reader.committed[1].Offset)
		require.Empty(t, logger.errors)
	})

	t.Run("fetch error is logged and reading continues", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(
			fetchResult{err: errors.New("broker unavailable")},
			fetchResult{msg: validMessage(1)},
		)

		runReadMessage(t, c, ctx)

		require.Equal(t, []string{"failed to fetch message"}, logger.errors)
		require.Len(t, reader.committed, 1)
	})

	t.Run("invalid message is not committed", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(
			fetchResult{msg: kafka.Message{Offset: 1, Value: []byte("not json")}},
			fetchResult{msg: validMessage(2)},
		)

		runReadMessage(t, c, ctx)

		require.Equal(t, []string{"failed to map kafka message to domain"}, logger.errors)
		require.Len(t, reader.committed, 1)
		require.Equal(t, int64(2), reader.committed[0].Offset)
	})

	t.Run("commit error is logged", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(fetchResult{msg: validMessage(1)})
		reader.commitErr = errors.New("commit failed")

		runReadMessage(t, c, ctx)

		require.Equal(t, []string{"failed to commit message"}, logger.errors)
		require.Empty(t, reader.committed)
	})

	t.Run("context canceled from fetch stops reading without error log", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(fetchResult{err: context.Canceled})

		runReadMessage(t, c, ctx)

		require.Empty(t, logger.errors)
		require.Empty(t, reader.committed)
	})

	t.Run("already canceled context returns immediately", func(t *testing.T) {
		c, reader, logger, ctx := newTestConsumer(fetchResult{msg: validMessage(1)})
		reader.cancel()

		runReadMessage(t, c, ctx)

		require.Empty(t, logger.errors)
		require.Empty(t, reader.committed)
		require.Len(t, reader.results, 1)
	})
}

func TestConsumer_Close(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		c, reader, _, _ := newTestConsumer()

		require.NoError(t, c.Close())
		require.True(t, reader.closed)
	})

	t.Run("error", func(t *testing.T) {
		c, reader, _, _ := newTestConsumer()
		reader.closeErr = errors.New("close failed")

		require.ErrorIs(t, c.Close(), reader.closeErr)
	})
}
