package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
)

// Роль не приходит от клиента: любая регистрация даёт покупателя, и это же
// значение уходит в событие users.created.
func TestRegister_Success_AlwaysCustomer(t *testing.T) {
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var createdUser domain.User
	var createdEvent domain.OutboxEvent

	users := mocks.NewMockUserRepository(ctrl)
	users.EXPECT().CreateWithEvent(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u domain.User, e domain.OutboxEvent) error {
			createdUser = u
			createdEvent = e
			return nil
		})
	hasher := mocks.NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Hash(gomock.Any()).
		DoAndReturn(func(plain string) (string, error) { return "hashed:" + plain, nil })

	svc := app.NewService(users, nil, hasher, nil, nil, nil, refreshTTL,
		app.WithNow(func() time.Time { return fixedNow }))

	got, err := svc.Register(context.Background(), dto.RegisterInput{
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
		t.Errorf("want role %q, got %q", domain.RoleCustomer, got.Role)
	}
	if !got.CreatedAt.Equal(fixedNow) {
		t.Errorf("want created at %v, got %v", fixedNow, got.CreatedAt)
	}

	if got.ID.Version() != 7 {
		t.Errorf("user ID must be UUIDv7, got %s", got.ID)
	}
	if createdUser.ID != got.ID || createdUser.Role != domain.RoleCustomer {
		t.Errorf("repository got unexpected user: %+v", createdUser)
	}

	if createdEvent.Topic != "users.created" {
		t.Errorf("event topic = %q, want users.created", createdEvent.Topic)
	}
	if createdEvent.Key != got.ID {
		t.Errorf("event key = %s, want user id %s", createdEvent.Key, got.ID)
	}
	var payload domain.UserCreatedPayload
	if err := json.Unmarshal(createdEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.EventID != createdEvent.ID || payload.UserID != got.ID ||
		payload.Email != "alice@example.com" || payload.Role != string(domain.RoleCustomer) ||
		!payload.OccurredAt.Equal(fixedNow) {
		t.Errorf("unexpected payload: %+v", payload)
	}
}

func TestRegister_InvalidPassword_DoesNotTouchRepository(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRepository(ctrl)
	svc := app.NewService(users, nil, mocks.NewMockPasswordHasher(ctrl), nil, nil, nil, refreshTTL)

	_, err := svc.Register(context.Background(), dto.RegisterInput{
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
	users.EXPECT().CreateWithEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(domain.ErrEmailTaken)
	hasher := mocks.NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Hash(gomock.Any()).Return("hashed", nil)

	svc := app.NewService(users, nil, hasher, nil, nil, nil, refreshTTL)

	_, err := svc.Register(context.Background(), dto.RegisterInput{
		Email:    "alice@example.com",
		Password: "password1",
	})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}
