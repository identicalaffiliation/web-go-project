package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

// testMaxAttempts — порог попыток, одинаковый во всех тестах outbox.
const testMaxAttempts = 5

func newTestUser(email string) domain.User {
	return domain.User{
		ID:           uuid.New(),
		Email:        domain.Email(email),
		PasswordHash: "hash",
		Role:         domain.RoleCustomer,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
}

func newTestEvent(user domain.User) domain.OutboxEvent {
	return domain.OutboxEvent{
		ID:        uuid.New(),
		Topic:     "users.created",
		Key:       user.ID,
		Payload:   []byte(`{"user_id":"` + user.ID.String() + `"}`),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	// table приходит только из тестов, не от пользователя.
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestCreateWithEvent_WritesBoth(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)

	user := newTestUser("alice@example.com")
	if err := repo.CreateWithEvent(context.Background(), user, newTestEvent(user)); err != nil {
		t.Fatalf("CreateWithEvent: %v", err)
	}

	if got := countRows(t, pool, "auth_users"); got != 1 {
		t.Errorf("auth_users = %d, want 1", got)
	}
	if got := countRows(t, pool, "outbox"); got != 1 {
		t.Errorf("outbox = %d, want 1", got)
	}
}

// Падает вторая вставка (повтор outbox.id): откатиться должна и первая.
func TestCreateWithEvent_OutboxFailure_RollsBackUser(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()

	first := newTestUser("first@example.com")
	event := newTestEvent(first)
	if err := repo.CreateWithEvent(ctx, first, event); err != nil {
		t.Fatalf("first CreateWithEvent: %v", err)
	}

	second := newTestUser("second@example.com")
	dupEvent := newTestEvent(second)
	dupEvent.ID = event.ID // конфликт первичного ключа outbox

	if err := repo.CreateWithEvent(ctx, second, dupEvent); err == nil {
		t.Fatal("expected error on duplicate outbox id")
	}

	if _, err := repo.GetByEmail(ctx, second.Email); err == nil {
		t.Error("second user must be rolled back")
	}
	if got := countRows(t, pool, "auth_users"); got != 1 {
		t.Errorf("auth_users = %d, want 1", got)
	}
	if got := countRows(t, pool, "outbox"); got != 1 {
		t.Errorf("outbox = %d, want 1", got)
	}
}

// Падает первая вставка (дубль email): события в outbox быть не должно.
func TestCreateWithEvent_DuplicateEmail_NoOutboxRow(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()

	first := newTestUser("dup@example.com")
	if err := repo.CreateWithEvent(ctx, first, newTestEvent(first)); err != nil {
		t.Fatalf("first CreateWithEvent: %v", err)
	}

	second := newTestUser("dup@example.com")
	err := repo.CreateWithEvent(ctx, second, newTestEvent(second))
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}

	if got := countRows(t, pool, "outbox"); got != 1 {
		t.Errorf("outbox = %d, want 1 (only the first event)", got)
	}
}

func seedEvents(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	repo := postgres.NewUserRepository(pool)
	for i := range n {
		u := newTestUser("user" + uuid.NewString() + "@example.com")
		u.CreatedAt = u.CreatedAt.Add(time.Duration(i) * time.Millisecond)
		e := newTestEvent(u)
		e.CreatedAt = u.CreatedAt
		if err := repo.CreateWithEvent(context.Background(), u, e); err != nil {
			t.Fatalf("seed event %d: %v", i, err)
		}
	}
}

// Пока первая транзакция держит строки, вторая не должна их получить.
func TestOutboxStore_SkipLocked(t *testing.T) {
	pool := setupPool(t)
	store := postgres.NewOutboxStore(pool)
	ctx := context.Background()
	seedEvents(t, pool, 3)

	b1, err := store.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin 1: %v", err)
	}
	defer func() { _ = b1.Rollback(ctx) }()

	first, err := b1.Fetch(ctx, 2, testMaxAttempts)
	if err != nil || len(first) != 2 {
		t.Fatalf("Fetch 1: got %d events, err %v, want 2", len(first), err)
	}

	b2, err := store.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin 2: %v", err)
	}
	defer func() { _ = b2.Rollback(ctx) }()

	// Если бы SKIP LOCKED не работал, Fetch завис бы в ожидании блокировки.
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	second, err := b2.Fetch(fetchCtx, 10, testMaxAttempts)
	if err != nil {
		t.Fatalf("Fetch 2: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("Fetch 2: got %d events, want the 1 unlocked", len(second))
	}
	for _, e := range first {
		if e.ID == second[0].ID {
			t.Fatalf("event %s taken by both transactions", e.ID)
		}
	}
}

// Два конкурентных «воркера» обрабатывают всё без пересечений и потерь.
func TestOutboxStore_ConcurrentWorkers_NoOverlap(t *testing.T) {
	pool := setupPool(t)
	store := postgres.NewOutboxStore(pool)
	ctx := context.Background()
	const total = 20
	seedEvents(t, pool, total)

	var (
		mu   sync.Mutex
		seen = make(map[uuid.UUID]int)
		wg   sync.WaitGroup
	)

	worker := func() {
		defer wg.Done()
		for {
			batch, err := store.Begin(ctx)
			if err != nil {
				t.Errorf("Begin: %v", err)
				return
			}
			events, err := batch.Fetch(ctx, 3, testMaxAttempts)
			if err != nil {
				_ = batch.Rollback(ctx)
				t.Errorf("Fetch: %v", err)
				return
			}
			if len(events) == 0 {
				_ = batch.Rollback(ctx)
				return
			}

			ids := make([]uuid.UUID, len(events))
			mu.Lock()
			for i, e := range events {
				ids[i] = e.ID
				seen[e.ID]++
			}
			mu.Unlock()

			if err := batch.MarkSent(ctx, ids); err != nil {
				_ = batch.Rollback(ctx)
				t.Errorf("MarkSent: %v", err)
				return
			}
			if err := batch.Commit(ctx); err != nil {
				t.Errorf("Commit: %v", err)
				return
			}
		}
	}

	wg.Add(2)
	go worker()
	go worker()
	wg.Wait()

	if len(seen) != total {
		t.Errorf("processed %d distinct events, want %d", len(seen), total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("event %s processed %d times, want 1", id, n)
		}
	}
}

func TestOutboxStore_MarkFailed_IncrementsAttemptsAndKeepsUnsent(t *testing.T) {
	pool := setupPool(t)
	store := postgres.NewOutboxStore(pool)
	ctx := context.Background()
	seedEvents(t, pool, 1)

	batch, err := store.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	events, err := batch.Fetch(ctx, 10, testMaxAttempts)
	if err != nil || len(events) != 1 {
		t.Fatalf("Fetch: %d events, err %v", len(events), err)
	}
	if err := batch.MarkFailed(ctx, []uuid.UUID{events[0].ID}); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if err := batch.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var attempts int
	var sentAtNull bool
	err = pool.QueryRow(ctx, `SELECT attempts, sent_at IS NULL FROM outbox WHERE id = $1`, events[0].ID).
		Scan(&attempts, &sentAtNull)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if attempts != 1 || !sentAtNull {
		t.Errorf("attempts=%d sentAtNull=%v, want 1/true", attempts, sentAtNull)
	}
}
