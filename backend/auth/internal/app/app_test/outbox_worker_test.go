package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

const testMaxAttempts = 5

var testWorkerCfg = app.OutboxWorkerConfig{
	Interval:       5 * time.Millisecond,
	BatchSize:      10,
	PublishTimeout: time.Second,
	MaxAttempts:    testMaxAttempts,
}

type workerFixture struct {
	store     *mocks.MockOutboxStore
	batch     *mocks.MockOutboxBatch
	publisher *mocks.MockEventPublisher
	log       *mocks.MockLogger
	worker    *app.OutboxWorker
}

// newWorkerFixture создаёт воркер с моками. Если errorLogExpected == false, любой
// вызов log.Error валит тест: так проверяем, что лишних Error-логов нет.
func newWorkerFixture(t *testing.T, errorLogExpected bool) *workerFixture {
	t.Helper()
	ctrl := gomock.NewController(t)

	f := &workerFixture{
		store:     mocks.NewMockOutboxStore(ctrl),
		batch:     mocks.NewMockOutboxBatch(ctrl),
		publisher: mocks.NewMockEventPublisher(ctrl),
		log:       mocks.NewMockLogger(ctrl),
	}
	// AnyTimes разрешает любое число вызовов, в том числе нулевое.
	f.log.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	f.log.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	f.log.EXPECT().WithError(gomock.Any()).Return(f.log).AnyTimes()
	if errorLogExpected {
		f.log.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
	}

	f.worker = app.NewOutboxWorker(f.store, f.publisher, ports.Logger(f.log), testWorkerCfg)
	return f
}

// signal даёт безопасно закрыть канал несколько раз (из разных тиков).
type signal struct {
	once sync.Once
	ch   chan struct{}
}

func newSignal() *signal { return &signal{ch: make(chan struct{})} }

func (s *signal) fire() { s.once.Do(func() { close(s.ch) }) }

// runUntil запускает воркер в горутине и отменяет контекст, когда сработал сигнал.
// Возвращает ошибку Run после полной остановки воркера.
func runUntil(t *testing.T, w *app.OutboxWorker, done *signal) error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	select {
	case <-done.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for worker to reach the expected state")
	}
	cancel()

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancel")
		return nil
	}
}

func testEvents(attempts ...int) []domain.OutboxEvent {
	events := make([]domain.OutboxEvent, len(attempts))
	for i, a := range attempts {
		events[i] = domain.OutboxEvent{
			ID: uuid.New(), Topic: "users.created", Key: uuid.New(), Payload: []byte(`{}`), Attempts: a,
		}
	}
	return events
}

func TestOutboxWorker_Success_MarksSentAndCommits(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(0, 0)
	done := newSignal()

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), testWorkerCfg.BatchSize, testMaxAttempts).Return(events, nil).Times(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).Return(nil).Times(1)
	f.batch.EXPECT().MarkSent(gomock.Any(), []uuid.UUID{events[0].ID, events[1].ID}).Return(nil).Times(1)
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// Общий сбой (Kafka недоступна) не должен тратить попытки: ни MarkFailed, ни
// Commit; транзакция откатывается. После восстановления события доставляются.
func TestOutboxWorker_KafkaDown_NoAttemptsThenRecovers(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(0, 0)
	done := newSignal()
	publishCalls := 0

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(events, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).DoAndReturn(
		func(context.Context, []domain.OutboxEvent) error {
			publishCalls++
			if publishCalls <= 3 {
				return errors.New("kafka down")
			}
			return nil
		},
	).MinTimes(4)
	// MarkFailed без EXPECT: любой вызов провалит тест.
	f.batch.EXPECT().MarkSent(gomock.Any(), []uuid.UUID{events[0].ID, events[1].ID}).Return(nil).Times(1)
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run must survive publish errors, got: %v", err)
	}
	if publishCalls < 4 {
		t.Errorf("publish calls = %d, want >= 4 (3 failures + success)", publishCalls)
	}
}

// Одно отклонённое брокером событие: остальные получают sent_at, ему attempts++.
func TestOutboxWorker_PoisonEvent_OthersSentPoisonFailed(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(0, 0, 0)
	poison := events[1]
	done := newSignal()

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(events, nil).Times(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).Return(&ports.PartialPublishError{
		Rejected: map[uuid.UUID]error{poison.ID: errors.New("message too large")},
	}).Times(1)
	f.batch.EXPECT().MarkSent(gomock.Any(), []uuid.UUID{events[0].ID, events[2].ID}).Return(nil).Times(1)
	f.batch.EXPECT().MarkFailed(gomock.Any(), []uuid.UUID{poison.ID}).Return(nil).Times(1)
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// Недоставленные из-за связи события в частичном результате не получают attempts.
func TestOutboxWorker_PartialUndelivered_NoAttempts(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(0, 0)
	done := newSignal()

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(events, nil).Times(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).Return(&ports.PartialPublishError{
		Undelivered: map[uuid.UUID]error{events[1].ID: errors.New("leader not available")},
	}).Times(1)
	f.batch.EXPECT().MarkSent(gomock.Any(), []uuid.UUID{events[0].ID}).Return(nil).Times(1)
	// MarkFailed без EXPECT: недоставленному событию попытка не засчитывается.
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// Пятая неудача пишет ровно один Error с event_id, topic и attempts (без payload).
func TestOutboxWorker_ReachesMaxAttempts_LogsErrorOnce(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(testMaxAttempts - 1)
	done := newSignal()

	f.log.EXPECT().Error("outbox event reached max attempts",
		"event_id", events[0].ID, "topic", "users.created", "attempts", testMaxAttempts).Times(1)

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(events, nil).Times(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).Return(&ports.PartialPublishError{
		Rejected: map[uuid.UUID]error{events[0].ID: errors.New("message too large")},
	}).Times(1)
	f.batch.EXPECT().MarkFailed(gomock.Any(), []uuid.UUID{events[0].ID}).Return(nil).Times(1)
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// До порога Error-лога быть не должно (в фикстуре без errorLogExpected он валит тест).
func TestOutboxWorker_BelowMaxAttempts_NoErrorLog(t *testing.T) {
	f := newWorkerFixture(t, false)
	events := testEvents(1)
	done := newSignal()

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(events, nil).Times(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.publisher.EXPECT().Publish(gomock.Any(), events).Return(&ports.PartialPublishError{
		Rejected: map[uuid.UUID]error{events[0].ID: errors.New("message too large")},
	}).Times(1)
	f.batch.EXPECT().MarkFailed(gomock.Any(), []uuid.UUID{events[0].ID}).Return(nil).Times(1)
	f.batch.EXPECT().Commit(gomock.Any()).DoAndReturn(func(context.Context) error {
		done.fire()
		return nil
	}).Times(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestOutboxWorker_FetchError_KeepsRunning(t *testing.T) {
	f := newWorkerFixture(t, true)
	done := newSignal()
	calls := 0

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(2)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, int, int) ([]domain.OutboxEvent, error) {
			calls++
			if calls == 2 {
				done.fire()
			}
			return nil, errors.New("db error")
		},
	).MinTimes(2)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()

	// Дошли до второго Fetch после ошибки первого: воркер не вышел.
	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestOutboxWorker_EmptyBatch_DoesNotPublish(t *testing.T) {
	f := newWorkerFixture(t, false)
	done := newSignal()

	f.store.EXPECT().Begin(gomock.Any()).Return(f.batch, nil).MinTimes(1)
	f.batch.EXPECT().Fetch(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, int, int) ([]domain.OutboxEvent, error) {
			done.fire()
			return nil, nil
		},
	).MinTimes(1)
	f.batch.EXPECT().Rollback(gomock.Any()).Return(nil).AnyTimes()
	// Publish без EXPECT: вызов провалит тест.

	if err := runUntil(t, f.worker, done); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestOutboxWorker_StopsOnContextCancel(t *testing.T) {
	f := newWorkerFixture(t, false)
	// Ни одного EXPECT на store: отменённый заранее контекст не должен
	// дойти до обработки батча.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- f.worker.Run(ctx) }()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run on cancelled ctx: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}
