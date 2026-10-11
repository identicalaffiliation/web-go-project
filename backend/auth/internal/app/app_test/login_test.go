package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
)

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	user := domain.User{ID: testUserID, Email: "alice@example.com", PasswordHash: "hashed", Role: domain.RoleCustomer}

	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().GetByEmail(gomock.Any(), domain.Email("alice@example.com")).Return(user, nil)

	hasher := mocks.NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Compare("hashed", "password1").Return(nil)

	issuer := mocks.NewMockAccessTokenIssuer(ctrl)
	issuer.EXPECT().Issue(gomock.Any()).Return("access-token", fixedNow.Add(accessTTL), nil)

	tokens := mocks.NewMockRefreshTokenGenerator(ctrl)
	tokens.EXPECT().Generate().Return("plain-refresh", "hash-refresh", nil)

	var storedToken domain.RefreshToken
	refreshes := mocks.NewMockRefreshTokenRepository(ctrl)
	refreshes.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, t domain.RefreshToken) error {
			storedToken = t
			return nil
		})

	svc := app.NewService(users, refreshes, hasher, issuer, nil, tokens, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

	pair, err := svc.Login(context.Background(), dto.LoginInput{Email: "alice@example.com", Password: "password1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pair.AccessToken != "access-token" || pair.RefreshToken != "plain-refresh" {
		t.Errorf("unexpected token pair: %+v", pair)
	}
	if storedToken.TokenHash != "hash-refresh" || storedToken.UserID != testUserID {
		t.Errorf("unexpected stored refresh token: %+v", storedToken)
	}
	if storedToken.ID.Version() != 7 {
		t.Errorf("refresh token ID must be UUIDv7, got %s", storedToken.ID)
	}
	if want := fixedNow.Add(refreshTTL); !pair.RefreshExpiresAt.Equal(want) || !storedToken.ExpiresAt.Equal(want) {
		t.Errorf("refresh expiry: pair=%v stored=%v, want %v", pair.RefreshExpiresAt, storedToken.ExpiresAt, want)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().GetByEmail(gomock.Any(), gomock.Any()).Return(domain.User{PasswordHash: "hashed"}, nil)
	hasher := mocks.NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Compare(gomock.Any(), gomock.Any()).Return(domain.ErrInvalidCredentials)

	svc := app.NewService(users, nil, hasher, nil, nil, nil, refreshTTL)

	_, err := svc.Login(context.Background(), dto.LoginInput{Email: "alice@example.com", Password: "wrong"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail_DoesNotLeakUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().GetByEmail(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrUserNotFound)

	svc := app.NewService(users, nil, mocks.NewMockPasswordHasher(ctrl), nil, nil, nil, refreshTTL)

	_, err := svc.Login(context.Background(), dto.LoginInput{Email: "nobody@example.com", Password: "whatever1"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if errors.Is(err, domain.ErrUserNotFound) {
		t.Fatal("ErrUserNotFound must not leak out of Login")
	}
}
