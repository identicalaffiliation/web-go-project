package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/postgres"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func TestRefreshTokenRepository_CreateAndGetByHash(t *testing.T) {
	pool := setupPool(t)
	users := postgres.NewUserRepository(pool)
	repo := postgres.NewRefreshTokenRepository(pool)
	ctx := context.Background()

	user := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Email:        "carol@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleCustomer,
		CreatedAt:    time.Now().UTC(),
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	token := domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: "hash-1",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UTC(),
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != token.ID || got.UserID != token.UserID {
		t.Errorf("GetByHash: got %+v, want %+v", got, token)
	}
}

func TestRefreshTokenRepository_GetByHash_NotFound(t *testing.T) {
	pool := setupPool(t)
	repo := postgres.NewRefreshTokenRepository(pool)

	_, err := repo.GetByHash(context.Background(), "no-such-hash")
	if !errors.Is(err, domain.ErrRefreshTokenNotFound) {
		t.Fatalf("want ErrRefreshTokenNotFound, got %v", err)
	}
}

func TestRefreshTokenRepository_Rotate(t *testing.T) {
	pool := setupPool(t)
	users := postgres.NewUserRepository(pool)
	repo := postgres.NewRefreshTokenRepository(pool)
	ctx := context.Background()

	user := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Email:        "dave@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleCustomer,
		CreatedAt:    time.Now().UTC(),
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	old := domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: "old-hash",
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, old); err != nil {
		t.Fatalf("create old token: %v", err)
	}

	next := domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: "new-hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UTC(),
		CreatedAt: time.Now().UTC(),
	}

	now := time.Now().UTC()
	if err := repo.Rotate(ctx, old.ID, next, now); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	gotOld, err := repo.GetByHash(ctx, old.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash(old): %v", err)
	}
	if gotOld.RevokedAt == nil {
		t.Error("old token must be revoked after Rotate")
	}

	gotNew, err := repo.GetByHash(ctx, next.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash(new): %v", err)
	}
	if gotNew.RevokedAt != nil {
		t.Error("new token must not be revoked")
	}
}
