//go:build unit

package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessEvents(t *testing.T) {
	ctx := context.Background()
	events := []*domain.Event{
		{EventID: uuid.NewV7(), Key: uuid.NewV7(), Payload: []byte(`{}`)},
		{EventID: uuid.NewV7(), Key: uuid.NewV7(), Payload: []byte(`{}`)},
	}
	ids := []uuid.UUID{events[0].EventID, events[1].EventID}

	t.Run("ok - events sent and marked", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		gomock.InOrder(
			repo.EXPECT().GetEvents(ctx, int64(10)).Return(events, nil),
			producer.EXPECT().SendMessages(ctx, events).Return(nil),
			repo.EXPECT().SentEvent(ctx, ids).Return(nil),
		)

		require.NoError(t, processEvents(ctx, repo, producer, 10))
	})

	t.Run("empty batch - nothing sent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		repo.EXPECT().GetEvents(ctx, int64(10)).Return(nil, nil)

		require.NoError(t, processEvents(ctx, repo, producer, 10))
	})

	t.Run("get events error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		repo.EXPECT().GetEvents(ctx, int64(10)).Return(nil, errors.New("db down"))

		require.Error(t, processEvents(ctx, repo, producer, 10))
	})

	t.Run("send error - events not marked as sent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		repo.EXPECT().GetEvents(ctx, int64(10)).Return(events, nil)
		producer.EXPECT().SendMessages(ctx, events).Return(errors.New("kafka down"))

		require.Error(t, processEvents(ctx, repo, producer, 10))
	})

	t.Run("mark sent error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		repo.EXPECT().GetEvents(ctx, int64(10)).Return(events, nil)
		producer.EXPECT().SendMessages(ctx, events).Return(nil)
		repo.EXPECT().SentEvent(ctx, ids).Return(errors.New("db down"))

		require.Error(t, processEvents(ctx, repo, producer, 10))
	})
}

func TestRun(t *testing.T) {
	cfg := &config.OutboxWorkerConfig{TickTime: 5 * time.Millisecond, Limit: 10}

	t.Run("processes on tick and stops on cancel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		ticked := make(chan struct{})
		var once sync.Once

		repo.EXPECT().GetEvents(gomock.Any(), int64(10)).DoAndReturn(
			func(context.Context, int64) ([]*domain.Event, error) {
				once.Do(func() { close(ticked) })
				return nil, nil
			},
		).MinTimes(1)

		done := make(chan error, 1)
		go func() { done <- Run(ctx, repo, producer, logger.NewNop(), cfg) }()

		<-ticked
		cancel()

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("worker did not stop after cancel")
		}
	})

	t.Run("error on tick does not stop worker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockOutboxRepository(ctrl)
		producer := mocks.NewMockProducer(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		calls := make(chan struct{}, 10)
		repo.EXPECT().GetEvents(gomock.Any(), int64(10)).DoAndReturn(
			func(context.Context, int64) ([]*domain.Event, error) {
				select {
				case calls <- struct{}{}:
				default:
				}
				return nil, errors.New("db down")
			},
		).MinTimes(2)

		done := make(chan error, 1)
		go func() { done <- Run(ctx, repo, producer, logger.NewNop(), cfg) }()

		<-calls
		<-calls
		cancel()
		require.ErrorIs(t, <-done, context.Canceled)
	})
}
