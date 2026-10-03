package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("auth-db"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	applyMigration(t, ctx, pool)
	return pool
}

func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	matches, err := filepath.Glob("../../../../../migrator/migrations/auth/*.sql")
	if err != nil || len(matches) == 0 {
		t.Fatalf("find migration file: %v (matches=%v)", err, matches)
	}

	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}

	up, _, found := strings.Cut(string(raw), "-- +goose Down")
	if !found {
		t.Fatalf("migration file has no '-- +goose Down' marker")
	}

	if _, err := pool.Exec(ctx, up); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()

	user := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleCustomer,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byEmail, err := repo.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if byEmail.ID != user.ID || byEmail.Role != user.Role {
		t.Errorf("GetByEmail: got %+v, want %+v", byEmail, user)
	}

	byID, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Email != user.Email {
		t.Errorf("GetByID: got %+v, want %+v", byID, user)
	}
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)
	ctx := context.Background()

	user := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleCustomer,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dup := user
	dup.ID = domain.UserID(uuid.NewString())

	err := repo.Create(ctx, dup)
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewUserRepository(pool)

	_, err := repo.GetByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}
