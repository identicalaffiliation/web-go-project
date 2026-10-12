package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func TestLogout_RevokesExistingToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	stored := domain.RefreshToken{ID: testTokenID}
	var revokedID uuid.UUID

	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(stored, nil)
	refreshes.EXPECT().Revoke(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uuid.UUID, _ time.Time) error {
			revokedID = id
			return nil
		})
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).DoAndReturn(func(plain string) string { return "hash-of-" + plain })

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL)

	if err := svc.Logout(context.Background(), "plain-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revokedID != testTokenID {
		t.Errorf("want to revoke %s, got %s", testTokenID, revokedID)
	}
}

func TestLogout_UnknownToken_IsNotAnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(domain.RefreshToken{}, ports.ErrRefreshTokenNotFound)
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).DoAndReturn(func(plain string) string { return "hash-of-" + plain })

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL)

	if err := svc.Logout(context.Background(), "unknown-token"); err != nil {
		t.Fatalf("Logout of an unknown token must not error, got %v", err)
	}
}
