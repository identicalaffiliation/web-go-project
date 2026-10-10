package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func TestLogout_RevokesExistingToken(t *testing.T) {
	stored := domain.RefreshToken{ID: "token-1"}
	var revokedID string

	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) { return stored, nil },
		revokeFn: func(_ context.Context, id string, _ time.Time) error {
			revokedID = id
			return nil
		},
	}
	tokens := tokenGenMock{hashFn: func(plain string) string { return "hash-of-" + plain }}
	clock := clockMock{nowFn: time.Now}

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, clock, nil, refreshTTL)

	if err := svc.Logout(context.Background(), "plain-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revokedID != "token-1" {
		t.Errorf("want to revoke token-1, got %s", revokedID)
	}
}

func TestLogout_UnknownToken_IsNotAnError(t *testing.T) {
	refreshes := &refreshRepoMock{
		getByHashFn: func(context.Context, string) (domain.RefreshToken, error) {
			return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
		},
	}
	tokens := tokenGenMock{hashFn: func(plain string) string { return "hash-of-" + plain }}

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, nil, nil, refreshTTL)

	if err := svc.Logout(context.Background(), "unknown-token"); err != nil {
		t.Fatalf("Logout of an unknown token must not error, got %v", err)
	}
}
