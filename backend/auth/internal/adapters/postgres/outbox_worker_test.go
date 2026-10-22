package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/logging"
	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// fakePublisher подменяет Kafka: поведение задаётся функцией, вызовы считаются.
type fakePublisher struct {
	mu        sync.Mutex
	calls     int
	published map[uuid.UUID]int
	behave    func(call int, events []domain.OutboxEvent) error
}

func (f *fakePublisher) Publish(_ context.Context, events []domain.OutboxEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.published == nil {
		f.published = make(map[uuid.UUID]int)
	}
	err := f.behave(f.calls, events)
	var partial *ports.PartialPublishError
	for _, e := range events {
		if err == nil || (errors.As(err, &partial) && !hasID(partial, e.ID)) {
			f.published[e.ID]++
		}
	}
	return err
}

func hasID(p *ports.PartialPublishError, id uuid.UUID) bool {
	_, rej := p.Rejected[id]
	_, und := p.Undelivered[id]
	return rej || und
}

func (f *fakePublisher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func startWorker(t *testing.T, pool *pgxpool.Pool, pub ports.EventPublisher) {
	t.Helper()

	worker := app.NewOutboxWorker(postgres.NewOutboxStore(pool), pub, logging.NewNop(), app.OutboxWorkerConfig{
		Interval:       10 * time.Millisecond,
		BatchSize:      10,
		PublishTimeout: 5 * time.Second,
		MaxAttempts:    testMaxAttempts,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", what)
}

type outboxRow struct {
	attempts int
	sent     bool
}

func rowState(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) outboxRow {
	t.Helper()
	var r outboxRow
	err := pool.QueryRow(context.Background(),
		`SELECT attempts, sent_at IS NOT NULL FROM outbox WHERE id = $1`, id).Scan(&r.attempts, &r.sent)
	if err != nil {
		t.Fatalf("select outbox row: %v", err)
	}
	return r
}

func outboxIDs(t *testing.T, pool *pgxpool.Pool) []uuid.UUID {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id FROM outbox ORDER BY created_at`)
	if err != nil {
		t.Fatalf("select ids: %v", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// Kafka недоступна несколько тиков: попытки не растут, после восстановления
// всё доставлено.
func TestOutboxWorker_KafkaDown_AttemptsUntouchedThenDelivered(t *testing.T) {
	pool := setupPool(t)
	seedEvents(t, pool, 3)
	ids := outboxIDs(t, pool)

	pub := &fakePublisher{behave: func(call int, _ []domain.OutboxEvent) error {
		if call <= 5 {
			return errors.New("kafka unavailable")
		}
		return nil
	}}
	startWorker(t, pool, pub)

	waitFor(t, "all events delivered", func() bool {
		for _, id := range ids {
			if !rowState(t, pool, id).sent {
				return false
			}
		}
		return true
	})

	for _, id := range ids {
		if got := rowState(t, pool, id).attempts; got != 0 {
			t.Errorf("event %s attempts = %d, want 0 (outage must not burn attempts)", id, got)
		}
	}
	if pub.callCount() < 6 {
		t.Errorf("publish calls = %d, want >= 6", pub.callCount())
	}
}

// Одно ядовитое событие: остальные получают sent_at, у ядовитого attempts растут
// до max, после чего Fetch его не выбирает и публикация прекращается.
func TestOutboxWorker_PoisonEvent_StopsAtMaxAttempts(t *testing.T) {
	pool := setupPool(t)
	seedEvents(t, pool, 3)
	ids := outboxIDs(t, pool)
	poison := ids[1]

	pub := &fakePublisher{behave: func(_ int, events []domain.OutboxEvent) error {
		for _, e := range events {
			if e.ID == poison {
				return &ports.PartialPublishError{Rejected: map[uuid.UUID]error{poison: errors.New("too large")}}
			}
		}
		return nil
	}}
	startWorker(t, pool, pub)

	waitFor(t, "poison reached max attempts", func() bool {
		return rowState(t, pool, poison).attempts == testMaxAttempts
	})

	for _, id := range []uuid.UUID{ids[0], ids[2]} {
		if !rowState(t, pool, id).sent {
			t.Errorf("healthy event %s must be sent", id)
		}
	}
	if st := rowState(t, pool, poison); st.sent || st.attempts != testMaxAttempts {
		t.Errorf("poison state = %+v, want unsent with %d attempts", st, testMaxAttempts)
	}

	// После порога Fetch не выбирает событие: новых публикаций быть не должно.
	before := pub.callCount()
	time.Sleep(200 * time.Millisecond)
	if after := pub.callCount(); after != before {
		t.Errorf("publish calls grew %d -> %d after max attempts", before, after)
	}
	if got := rowState(t, pool, poison).attempts; got != testMaxAttempts {
		t.Errorf("attempts = %d, want to stay %d", got, testMaxAttempts)
	}

	store := postgres.NewOutboxStore(pool)
	batch, err := store.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer func() { _ = batch.Rollback(context.Background()) }()
	events, err := batch.Fetch(context.Background(), 10, testMaxAttempts)
	if err != nil || len(events) != 0 {
		t.Errorf("Fetch after exhaustion: %d events, err %v, want none", len(events), err)
	}
}
