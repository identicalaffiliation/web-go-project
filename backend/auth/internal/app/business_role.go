package app

import (
	"context"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// BusinessRoleHandler выдаёт роль business пользователю, которому Profile
// создал бизнес-профиль. Новая роль попадёт в токен после /refresh: он берёт
// роль из БД, а /validate по-прежнему читает её только из access-токена.
type BusinessRoleHandler struct {
	users ports.UserRoleUpdater
}

func NewBusinessRoleHandler(users ports.UserRoleUpdater) *BusinessRoleHandler {
	return &BusinessRoleHandler{users: users}
}

// Handle обрабатывает тело сообщения. Ошибки не классифицирует, а возвращает
// обёрнутыми: domain.ErrInvalidEvent и ports.ErrUserNotFound consumer
// пропускает, всё остальное считает временным сбоем и повторяет.
// Идемпотентна: повторная доставка того же события ничего не ломает.
func (h *BusinessRoleHandler) Handle(ctx context.Context, value []byte) error {
	ev, err := domain.ParseBusinessProfileCreated(value)
	if err != nil {
		return err
	}

	if err := h.users.UpdateRole(ctx, ev.UserID, domain.RoleBusiness); err != nil {
		return fmt.Errorf("set business role for user %s (event %s): %w", ev.UserID, ev.EventID, err)
	}
	return nil
}
