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

func TestLogin_Success(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	user := domain.User{ID: "user-1", Email: "alice@example.com", PasswordHash: "hashed", Role: domain.RoleCustomer}

	users := &userRepoMock{
		getByEmailFn: func(_ context.Context, email domain.Email) (domain.User, error) {
			if email != "alice@example.com" {
				t.Fatalf("unexpected email: %s", email)
			}
			return user, nil
		},
	}
	hasher := hasherMock{compareFn: func(hash, plain string) error {
		if hash != "hashed" || plain != "password1" {
			t.Fatalf("unexpected compare args: %s %s", hash, plain)
		}
		return nil
	}}
	issuer := issuerMock{issueFn: func(c domain.Claims) (string, time.Time, error) {
		return "access-token", fixedNow.Add(app.AccessTokenTTL), nil
	}}
	tokens := tokenGenMock{generateFn: func() (string, string, error) { return "plain-refresh", "hash-refresh", nil }}
	var storedToken domain.RefreshToken
	refreshes := &refreshRepoMock{
		createFn: func(_ context.Context, t domain.RefreshToken) error {
			storedToken = t
			return nil
		},
	}
	clock := clockMock{nowFn: func() time.Time { return fixedNow }}
	ids := idGenMock{newIDFn: func() string { return "refresh-1" }}

	svc := app.NewService(users, refreshes, hasher, issuer, nil, tokens, clock, ids)

	pair, err := svc.Login(context.Background(), ports.LoginInput{Email: "alice@example.com", Password: "password1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair.AccessToken != "access-token" || pair.RefreshToken != "plain-refresh" {
		t.Errorf("unexpected token pair: %+v", pair)
	}
	if storedToken.TokenHash != "hash-refresh" || storedToken.UserID != "user-1" {
		t.Errorf("unexpected stored refresh token: %+v", storedToken)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	users := &userRepoMock{
		getByEmailFn: func(context.Context, domain.Email) (domain.User, error) {
			return domain.User{PasswordHash: "hashed"}, nil
		},
	}
	hasher := hasherMock{compareFn: func(string, string) error { return domain.ErrInvalidCredentials }}

	svc := app.NewService(users, nil, hasher, nil, nil, nil, nil, nil)

	_, err := svc.Login(context.Background(), ports.LoginInput{Email: "alice@example.com", Password: "wrong"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail_DoesNotLeakUserNotFound(t *testing.T) {
	users := &userRepoMock{
		getByEmailFn: func(context.Context, domain.Email) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}

	svc := app.NewService(users, nil, hasherMock{}, nil, nil, nil, nil, nil)

	_, err := svc.Login(context.Background(), ports.LoginInput{Email: "nobody@example.com", Password: "whatever1"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal("ErrUserNotFound must not leak out of Login")
	}
}
