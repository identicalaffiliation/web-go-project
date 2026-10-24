package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// fakeReader отдаёт сообщения по очереди, а когда они кончились, отменяет ctx
// и возвращает его ошибку: так Run завершается без горутин и sleep в тесте.
type fakeReader struct {
	queue     []kafka.Message
	cancel    context.CancelFunc
	committed []kafka.Message
	commitErr error
	closed    int
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if len(r.queue) == 0 {
		r.cancel()
		return kafka.Message{}, ctx.Err()
	}
	msg := r.queue[0]
	r.queue = r.queue[1:]
	return msg, nil
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	r.committed = append(r.committed, msgs...)
	return r.commitErr
}

func (r *fakeReader) Close() error {
	r.closed++
	return nil
}

// fakeHandler возвращает ошибки по порядку вызовов; когда они кончились — nil.
type fakeHandler struct {
	results []error
	calls   int
	onCall  func(call int)
}

func (h *fakeHandler) Handle(context.Context, []byte) error {
	h.calls++
	if h.onCall != nil {
		h.onCall(h.calls)
	}
	if h.calls > len(h.results) {
		return nil
	}
	return h.results[h.calls-1]
}

// fakeTopic возвращает результаты по порядку вызовов; когда они кончились — nil
// (топик есть).
type fakeTopic struct {
	results []error
	calls   int
	onCall  func(call int)
}

func (t *fakeTopic) Exists(context.Context) error {
	t.calls++
	if t.onCall != nil {
		t.onCall(t.calls)
	}
	if t.calls > len(t.results) {
		return nil
	}
	return t.results[t.calls-1]
}

// recLogger считает записи по уровням. With и WithError возвращают тот же
// объект: тестам нужны только счётчики.
type recLogger struct {
	mu     *sync.Mutex
	counts map[string]int
}

func newRecLogger() recLogger {
	return recLogger{mu: &sync.Mutex{}, counts: map[string]int{}}
}

func (l recLogger) rec(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[level]++
}

func (l recLogger) count(level string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[level]
}

func (l recLogger) Debug(string, ...any)         { l.rec("debug") }
func (l recLogger) Info(string, ...any)          { l.rec("info") }
func (l recLogger) Warn(string, ...any)          { l.rec("warn") }
func (l recLogger) Error(string, ...any)         { l.rec("error") }
func (l recLogger) With(...any) ports.Logger     { return l }
func (l recLogger) WithError(error) ports.Logger { return l }

func readerFactory(r messageReader, created *int) func() messageReader {
	return func() messageReader {
		if created != nil {
			*created++
		}
		return r
	}
}

func run(t *testing.T, msgs []kafka.Message, handler *fakeHandler) *fakeReader {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &fakeReader{queue: msgs, cancel: cancel}
	c := newConsumer(readerFactory(reader, nil), &fakeTopic{}, handler, newRecLogger(), time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil on shutdown", err)
	}
	return reader
}

func message(offset int64) kafka.Message {
	return kafka.Message{Partition: 0, Offset: offset, Value: []byte("{}")}
}

func TestConsumer_Success_Commits(t *testing.T) {
	handler := &fakeHandler{}

	reader := run(t, []kafka.Message{message(1)}, handler)

	if handler.calls != 1 || len(reader.committed) != 1 || reader.committed[0].Offset != 1 {
		t.Fatalf("calls=%d committed=%v, want one call and offset 1 committed", handler.calls, reader.committed)
	}
}

func TestConsumer_InvalidEvent_SkippedAndCommitted(t *testing.T) {
	handler := &fakeHandler{results: []error{fmt.Errorf("%w: malformed json", domain.ErrInvalidEvent)}}

	reader := run(t, []kafka.Message{message(1)}, handler)

	if handler.calls != 1 || len(reader.committed) != 1 {
		t.Fatalf("calls=%d committed=%d, want 1 and 1", handler.calls, len(reader.committed))
	}
}

func TestConsumer_UserNotFound_SkippedAndCommitted(t *testing.T) {
	handler := &fakeHandler{results: []error{fmt.Errorf("set role: %w", ports.ErrUserNotFound)}}

	reader := run(t, []kafka.Message{message(1)}, handler)

	if handler.calls != 1 || len(reader.committed) != 1 {
		t.Fatalf("calls=%d committed=%d, want 1 and 1", handler.calls, len(reader.committed))
	}
}

// Ошибка БД: сообщение повторяется, оффсет коммитится один раз и только после успеха.
func TestConsumer_DBError_RetriesThenCommits(t *testing.T) {
	dbErr := errors.New("connection refused")
	handler := &fakeHandler{results: []error{dbErr, dbErr}}

	reader := run(t, []kafka.Message{message(1), message(2)}, handler)

	// Сообщение 1: два сбоя и успех; сообщение 2: успех.
	if handler.calls != 4 {
		t.Fatalf("handler calls = %d, want 4", handler.calls)
	}
	if len(reader.committed) != 2 || reader.committed[0].Offset != 1 || reader.committed[1].Offset != 2 {
		t.Fatalf("committed = %v, want offsets 1 and 2 in order", reader.committed)
	}
}

// Пока БД недоступна, оффсет не коммитится, а остановка по ctx не считается ошибкой.
func TestConsumer_DBError_NoCommitUntilShutdown(t *testing.T) {
	dbErr := errors.New("connection refused")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &fakeHandler{
		results: []error{dbErr, dbErr, dbErr, dbErr, dbErr},
		onCall: func(call int) {
			if call == 3 {
				cancel()
			}
		},
	}
	reader := &fakeReader{queue: []kafka.Message{message(1), message(2)}, cancel: cancel}
	c := newConsumer(readerFactory(reader, nil), &fakeTopic{}, handler, newRecLogger(), time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil on shutdown", err)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("committed = %v, want nothing", reader.committed)
	}
	if len(reader.queue) != 1 {
		t.Fatalf("queue len = %d: message 2 must not be fetched while message 1 is failing", len(reader.queue))
	}
}

// Сбой коммита не должен ронять консьюмера.
func TestConsumer_CommitError_Continues(t *testing.T) {
	handler := &fakeHandler{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &fakeReader{
		queue:     []kafka.Message{message(1), message(2)},
		cancel:    cancel,
		commitErr: errors.New("broker unavailable"),
	}
	c := newConsumer(readerFactory(reader, nil), &fakeTopic{}, handler, newRecLogger(), time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	if handler.calls != 2 {
		t.Fatalf("handler calls = %d, want 2", handler.calls)
	}
}

// Пока топика нет, reader не создаётся; о нём пишется один Warn, а не по одному
// на каждую попытку, и один Info, когда топик появился.
func TestConsumer_WaitsForTopic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic := &fakeTopic{results: []error{ErrTopicNotFound, ErrTopicNotFound, ErrTopicNotFound}}
	var createdAtCheck []int
	created := 0
	reader := &fakeReader{cancel: cancel}
	log := newRecLogger()
	topic.onCall = func(call int) { createdAtCheck = append(createdAtCheck, created) }

	c := newConsumer(readerFactory(reader, &created), topic, &fakeHandler{}, log, time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	if topic.calls != 4 {
		t.Errorf("topic checks = %d, want 4 (3 misses and a hit)", topic.calls)
	}
	for i, n := range createdAtCheck {
		if n != 0 {
			t.Errorf("reader was created before check %d succeeded", i+1)
		}
	}
	if created != 1 {
		t.Errorf("readers created = %d, want 1", created)
	}
	if got := log.count("warn"); got != 1 {
		t.Errorf("warn logs = %d, want 1 for three consecutive misses", got)
	}
	if got := log.count("error"); got != 0 {
		t.Errorf("error logs = %d, want 0", got)
	}
}

// Лог пишется при смене причины ожидания: «топика нет» и «Kafka недоступна»
// различаются, но повторы одной причины молчат.
func TestConsumer_TopicWait_LogsOnReasonChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	down := errors.New("dial tcp: connection refused")
	topic := &fakeTopic{results: []error{down, down, down, ErrTopicNotFound, ErrTopicNotFound, down}}
	log := newRecLogger()
	reader := &fakeReader{cancel: cancel}

	c := newConsumer(readerFactory(reader, nil), topic, &fakeHandler{}, log, time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	// down -> missing -> down: три смены причины из шести неудачных проверок.
	if got := log.count("warn"); got != 3 {
		t.Errorf("warn logs = %d, want 3", got)
	}
}

func TestConsumer_ShutdownWhileWaitingForTopic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic := &fakeTopic{
		results: []error{ErrTopicNotFound, ErrTopicNotFound, ErrTopicNotFound, ErrTopicNotFound},
		onCall: func(call int) {
			if call == 3 {
				cancel()
			}
		},
	}
	created := 0
	c := newConsumer(readerFactory(&fakeReader{cancel: cancel}, &created), topic, &fakeHandler{},
		newRecLogger(), time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run returned %v, want nil on shutdown", err)
	}
	if created != 0 {
		t.Errorf("readers created = %d, want 0 when the topic never appeared", created)
	}
}

// Топика так и не было, reader не создавался: Close не должен падать на nil.
func TestConsumer_Close_NilReader(t *testing.T) {
	c := newConsumer(readerFactory(&fakeReader{}, nil), &fakeTopic{}, &fakeHandler{},
		newRecLogger(), time.Millisecond, time.Second)

	if err := c.Close(); err != nil {
		t.Fatalf("Close before Run returned %v, want nil", err)
	}
	// Повторный Close тоже безопасен.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close returned %v, want nil", err)
	}
}

// Close, вызванный до Run, не должен оставить после себя неосвобождённый reader.
func TestConsumer_Run_AfterClose_CreatesNoReader(t *testing.T) {
	created := 0
	c := newConsumer(readerFactory(&fakeReader{}, &created), &fakeTopic{}, &fakeHandler{},
		newRecLogger(), time.Millisecond, time.Second)

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	if created != 0 {
		t.Errorf("readers created = %d, want 0 after Close", created)
	}
}

func TestConsumer_Close_ClosesReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &fakeReader{cancel: cancel}
	c := newConsumer(readerFactory(reader, nil), &fakeTopic{}, &fakeHandler{}, newRecLogger(), time.Millisecond, time.Second)

	if err := c.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if reader.closed != 1 {
		t.Errorf("reader.Close calls = %d, want 1", reader.closed)
	}
}
