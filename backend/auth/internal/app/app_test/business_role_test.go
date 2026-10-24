package app_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/identicalaffiliation/web-go-project/auth/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/auth/internal/app"
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

const businessEvent = `{"event_id":"019a0000-0000-7000-8000-000000000009","user_id":"019a0000-0000-7000-8000-000000000001"}`

func TestBusinessRoleHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRoleUpdater(ctrl)
	users.EXPECT().UpdateRole(gomock.Any(), testUserID, domain.RoleBusiness).Return(nil)

	h := app.NewBusinessRoleHandler(users)

	if err := h.Handle(context.Background(), []byte(businessEvent)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Повторная доставка того же события — не ошибка.
func TestBusinessRoleHandler_Repeat_Idempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRoleUpdater(ctrl)
	users.EXPECT().UpdateRole(gomock.Any(), testUserID, domain.RoleBusiness).Return(nil).Times(2)

	h := app.NewBusinessRoleHandler(users)

	for range 2 {
		if err := h.Handle(context.Background(), []byte(businessEvent)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestBusinessRoleHandler_InvalidEvent(t *testing.T) {
	tests := map[string]string{
		"broken json":     `{"user_id":`,
		"missing user_id": `{"event_id":"019a0000-0000-7000-8000-000000000009"}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			// UpdateRole не ожидается: gomock упадёт, если обработчик его вызовет.
			users := mocks.NewMockUserRoleUpdater(gomock.NewController(t))
			h := app.NewBusinessRoleHandler(users)

			err := h.Handle(context.Background(), []byte(raw))
			if !errors.Is(err, domain.ErrInvalidEvent) {
				t.Fatalf("want ErrInvalidEvent, got %v", err)
			}
		})
	}
}

func TestBusinessRoleHandler_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRoleUpdater(ctrl)
	users.EXPECT().UpdateRole(gomock.Any(), testUserID, domain.RoleBusiness).Return(ports.ErrUserNotFound)

	h := app.NewBusinessRoleHandler(users)

	err := h.Handle(context.Background(), []byte(businessEvent))
	if !errors.Is(err, ports.ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestBusinessRoleHandler_DBError_Propagates(t *testing.T) {
	dbErr := errors.New("connection refused")
	ctrl := gomock.NewController(t)
	users := mocks.NewMockUserRoleUpdater(ctrl)
	users.EXPECT().UpdateRole(gomock.Any(), testUserID, domain.RoleBusiness).Return(dbErr)

	h := app.NewBusinessRoleHandler(users)

	err := h.Handle(context.Background(), []byte(businessEvent))
	if !errors.Is(err, dbErr) {
		t.Fatalf("want db error, got %v", err)
	}
	if errors.Is(err, domain.ErrInvalidEvent) || errors.Is(err, ports.ErrUserNotFound) {
		t.Fatalf("db error must not look skippable: %v", err)
	}
}
