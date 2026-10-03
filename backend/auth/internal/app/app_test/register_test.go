package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func TestRegister_Success(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var createdUser domain.User

	users := &userRepoMock{
		createFn: func(_ context.Context, u domain.User) error {
			createdUser = u
			return nil
		},
	}
	hasher := hasherMock{hashFn: func(plain string) (string, error) { return "hashed:" + plain, nil }}
	ids := idGenMock{newIDFn: func() string { return "user-1" }}
	clock := clockMock{nowFn: func() time.Time { return fixedNow }}

	svc := app.NewService(users, nil, hasher, nil, nil, nil, clock, ids)

	got, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "Alice@Example.com",
		Password: "password1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("email not normalized: got %q", got.Email)
	}
	if got.PasswordHash != "hashed:password1" {
		t.Errorf("unexpected password hash: got %q", got.PasswordHash)
	}
	if got.Role != domain.RoleCustomer {
		t.Errorf("want default role %q, got %q", domain.RoleCustomer, got.Role)
	}
	if createdUser.ID != "user-1" {
		t.Errorf("repository got unexpected user: %+v", createdUser)
	}
}

func TestRegister_InvalidPassword_DoesNotTouchRepository(t *testing.T) {
	users := &userRepoMock{
		createFn: func(context.Context, domain.User) error {
			t.Fatal("Create must not be called for an invalid password")
			return nil
		},
	}
	svc := app.NewService(users, nil, hasherMock{}, nil, nil, nil, nil, nil)

	_, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "alice@example.com",
		Password: "short",
	})
	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword, got %v", err)
	}
}

func TestRegister_EmailTaken_PropagatesFromRepository(t *testing.T) {
	users := &userRepoMock{
		createFn: func(context.Context, domain.User) error { return domain.ErrEmailTaken },
	}
	hasher := hasherMock{hashFn: func(plain string) (string, error) { return "hashed", nil }}
	ids := idGenMock{newIDFn: func() string { return "user-1" }}
	clock := clockMock{nowFn: time.Now}

	svc := app.NewService(users, nil, hasher, nil, nil, nil, clock, ids)

	_, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "alice@example.com",
		Password: "password1",
	})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}
