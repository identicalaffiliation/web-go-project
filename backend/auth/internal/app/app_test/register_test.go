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
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func TestRegister_Success(t *testing.T) {
	roles := []domain.Role{domain.RoleCustomer, domain.RoleBusiness}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			var createdUser domain.User

			users := mocks.NewMockUserRepository(ctrl)
			users.EXPECT().Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, u domain.User) error {
					createdUser = u
					return nil
				})
			hasher := mocks.NewMockPasswordHasher(ctrl)
			hasher.EXPECT().Hash(gomock.Any()).
				DoAndReturn(func(plain string) (string, error) { return "hashed:" + plain, nil })

			svc := app.NewService(users, nil, hasher, nil, nil, nil, refreshTTL,
				app.WithNow(func() time.Time { return fixedNow }))

			got, err := svc.Register(context.Background(), ports.RegisterInput{
				Email:    "Alice@Example.com",
				Password: "password1",
				Role:     string(role),
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
			if got.Role != role {
				t.Errorf("want role %q, got %q", role, got.Role)
			}
			if !got.CreatedAt.Equal(fixedNow) {
				t.Errorf("want created at %v, got %v", fixedNow, got.CreatedAt)
			}
			
			if got.ID.Version() != 7 {
				t.Errorf("user ID must be UUIDv7, got %s", got.ID)
			}
			if createdUser.ID != got.ID || createdUser.Role != role {
				t.Errorf("repository got unexpected user: %+v", createdUser)
			}
		})
	}
}

func TestRegister_InvalidRole_DoesNotTouchRepository(t *testing.T) {
	// Пустая роль тоже невалидна: дефолта нет.
	for _, role := range []string{"", "admin", "Customer"} {
		t.Run("role="+role, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// Без EXPECT() любой вызов мока валит тест.
			users := mocks.NewMockUserRepository(ctrl)
			svc := app.NewService(users, nil, mocks.NewMockPasswordHasher(ctrl), nil, nil, nil, refreshTTL)

			_, err := svc.Register(context.Background(), ports.RegisterInput{
				Email:    "alice@example.com",
				Password: "password1",
				Role:     role,
			})
			if !errors.Is(err, domain.ErrInvalidRole) {
				t.Fatalf("want ErrInvalidRole, got %v", err)
			}
		})
	}
}

func TestRegister_InvalidPassword_DoesNotTouchRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	svc := app.NewService(users, nil, mocks.NewMockPasswordHasher(ctrl), nil, nil, nil, refreshTTL)

	_, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "alice@example.com",
		Password: "short",
	})
	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword, got %v", err)
	}
}

func TestRegister_EmailTaken_PropagatesFromRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.ErrEmailTaken)
	hasher := mocks.NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Hash(gomock.Any()).Return("hashed", nil)

	svc := app.NewService(users, nil, hasher, nil, nil, nil, refreshTTL)

	_, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "alice@example.com",
		Password: "password1",
		Role:     string(domain.RoleCustomer),
	})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}
