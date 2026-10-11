package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func TestRefresh_Success_Rotates(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: testTokenID, UserID: testUserID, ExpiresAt: fixedNow.Add(time.Hour)}
	user := domain.User{ID: testUserID, Email: "alice@example.com", Role: domain.RoleCustomer}

	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) { return stored, nil },
		rotateFn: func(_ context.Context, oldID uuid.UUID, next domain.RefreshToken, _ time.Time) error {
			if oldID != testTokenID {
				t.Fatalf("want to rotate %s, got %s", testTokenID, oldID)
			}
			if next.TokenHash != "new-hash" || next.UserID != testUserID || next.ID == testTokenID {
				t.Fatalf("unexpected next token: %+v", next)
			}
			return nil
		},
	}
	users := &userRepoMock{
		getByIDFn: func(context.Context, uuid.UUID) (domain.User, error) { return user, nil },
	}
	issuer := issuerMock{issueFn: func(domain.Claims) (string, time.Time, error) {
		return "new-access", fixedNow.Add(accessTTL), nil
	}}
	tokens := tokenGenMock{
		hashFn:     func(plain string) string { return "hash-of-" + plain },
		generateFn: func() (string, string, error) { return "new-plain", "new-hash", nil },
	}

	svc := app.NewService(users, refreshes, nil, issuer, nil, tokens, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

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
	stored := domain.RefreshToken{ID: testTokenID, ExpiresAt: fixedNow.Add(-time.Minute)}

	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) { return stored, nil },
	}
	tokens := tokenGenMock{hashFn: func(plain string) string { return "hash-of-" + plain }}

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

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

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL)

	_, err := svc.Refresh(context.Background(), "unknown-token")
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
