package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func TestRefresh_Success_Rotates(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: "old-id", UserID: "user-1", ExpiresAt: fixedNow.Add(time.Hour)}
	user := domain.User{ID: "user-1", Email: "alice@example.com", Role: domain.RoleCustomer}

	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) { return stored, nil },
		rotateFn: func(_ context.Context, oldID string, next domain.RefreshToken, _ time.Time) error {
			if oldID != "old-id" {
				t.Fatalf("want to rotate old-id, got %s", oldID)
			}
			if next.TokenHash != "new-hash" {
				t.Fatalf("unexpected next token: %+v", next)
			}
			return nil
		},
	}
	users := &userRepoMock{
		getByIDFn: func(context.Context, domain.UserID) (domain.User, error) { return user, nil },
	}
	issuer := issuerMock{issueFn: func(domain.Claims) (string, time.Time, error) {
		return "new-access", fixedNow.Add(app.AccessTokenTTL), nil
	}}
	tokens := tokenGenMock{
		hashFn:     func(plain string) string { return "hash-of-" + plain },
		generateFn: func() (string, string, error) { return "new-plain", "new-hash", nil },
	}
	clock := clockMock{nowFn: func() time.Time { return fixedNow }}
	ids := idGenMock{newIDFn: func() string { return "new-id" }}

	svc := app.NewService(users, refreshes, nil, issuer, nil, tokens, clock, ids)

	pair, err := svc.Refresh(context.Background(), "old-plain-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair.AccessToken != "new-access" || pair.RefreshToken != "new-plain" {
		t.Errorf("unexpected token pair: %+v", pair)
	}
}

func TestRefresh_Expired(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: "old-id", ExpiresAt: fixedNow.Add(-time.Minute)}

	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) { return stored, nil },
	}
	tokens := tokenGenMock{hashFn: func(plain string) string { return "hash-of-" + plain }}
	clock := clockMock{nowFn: func() time.Time { return fixedNow }}

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, clock, nil)

	_, err := svc.Refresh(context.Background(), "old-plain-token")
	if !errors.Is(err, domain.ErrRefreshTokenExpired) {
		t.Fatalf("want ErrRefreshTokenExpired, got %v", err)
	}
}

func TestRefresh_NotFound_MapsToInvalidToken(t *testing.T) {
	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) {
			return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
		},
	}
	tokens := tokenGenMock{hashFn: func(plain string) string { return "hash-of-" + plain }}

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, nil, nil)

	_, err := svc.Refresh(context.Background(), "unknown-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
