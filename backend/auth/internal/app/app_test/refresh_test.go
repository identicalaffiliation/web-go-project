package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func TestRefresh_Success_Rotates(t *testing.T) {
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: testTokenID, UserID: testUserID, ExpiresAt: fixedNow.Add(time.Hour)}
	user := domain.User{ID: testUserID, Email: "alice@example.com", Role: domain.RoleCustomer}

	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(stored, nil)
	refreshes.EXPECT().Rotate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, oldID uuid.UUID, next domain.RefreshToken, _ time.Time) error {
			if oldID != testTokenID {
				t.Fatalf("want to rotate %s, got %s", testTokenID, oldID)
			}
			if next.TokenHash != "new-hash" || next.UserID != testUserID || next.ID == testTokenID {
				t.Fatalf("unexpected next token: %+v", next)
			}
			return nil
		})
	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(user, nil)
	issuer := mocks.NewMockAccessTokenIssuer(ctrl)
	issuer.EXPECT().Issue(gomock.Any()).Return("new-access", fixedNow.Add(accessTTL), nil)
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).DoAndReturn(func(plain string) string { return "hash-of-" + plain })
	tokens.EXPECT().Generate().Return("new-plain", "new-hash", nil)

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

// Роль для нового access-токена берётся из хранилища пользователей, а не из
// старого токена: после смены роли консьюмером /refresh выдаёт business.
func TestRefresh_UsesCurrentRoleFromStorage(t *testing.T) {
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: testTokenID, UserID: testUserID, ExpiresAt: fixedNow.Add(time.Hour)}
	// В хранилище уже business, хотя при регистрации пользователь был customer.
	user := domain.User{ID: testUserID, Email: "alice@example.com", Role: domain.RoleBusiness}

	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(stored, nil)
	refreshes.EXPECT().Rotate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().GetByID(gomock.Any(), testUserID).Return(user, nil)
	issuer := mocks.NewMockAccessTokenIssuer(ctrl)
	issuer.EXPECT().Issue(gomock.Any()).
		DoAndReturn(func(claims domain.Claims) (string, time.Time, error) {
			if claims.Role != domain.RoleBusiness {
				t.Errorf("claims.Role = %q, want %q", claims.Role, domain.RoleBusiness)
			}
			return "new-access", fixedNow.Add(accessTTL), nil
		})
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).Return("old-hash")
	tokens.EXPECT().Generate().Return("new-plain", "new-hash", nil)

	svc := app.NewService(users, refreshes, nil, issuer, nil, tokens, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

	if _, err := svc.Refresh(context.Background(), "old-plain-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefresh_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stored := domain.RefreshToken{ID: testTokenID, ExpiresAt: fixedNow.Add(-time.Minute)}

	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(stored, nil)
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).DoAndReturn(func(plain string) string { return "hash-of-" + plain })

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

	_, err := svc.Refresh(context.Background(), "old-plain-token")
	if !errors.Is(err, domain.ErrRefreshTokenExpired) {
		t.Fatalf("want ErrRefreshTokenExpired, got %v", err)
	}
}

func TestRefresh_NotFound_Propagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(domain.RefreshToken{}, ports.ErrRefreshTokenNotFound)
	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Hash(gomock.Any()).DoAndReturn(func(plain string) string { return "hash-of-" + plain })

	svc := app.NewService(nil, refreshes, nil, nil, nil, tokens, refreshTTL)

	_, err := svc.Refresh(context.Background(), "unknown-token")
	if !errors.Is(err, ports.ErrRefreshTokenNotFound) {
		t.Fatalf("want ErrRefreshTokenNotFound, got %v", err)
	}
}
