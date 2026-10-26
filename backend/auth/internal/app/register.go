package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
)

func (s *Service) Register(ctx context.Context, in dto.RegisterInput) (domain.User, error) {
	email, err := domain.NewEmail(in.Email)
	if err != nil {
		return domain.User{}, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return domain.User{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate user id: %w", err)
	}

	now := s.now()
	user := domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
		// Роль не приходит от клиента: иначе он выдал бы себе привилегии. Любая
		// регистрация даёт покупателя, а business выдаёт consumer по событию Profile.
		Role:      domain.RoleCustomer,
		CreatedAt: now,
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate event id: %w", err)
	}
	event, err := domain.NewUserCreatedEvent(user, eventID, s.userCreatedTopic, now)
	if err != nil {
		return domain.User{}, fmt.Errorf("build users.created event: %w", err)
	}

	// Событие пишется в той же транзакции, что и пользователь: иначе при сбое
	// между двумя записями Profile никогда не узнал бы о новом пользователе.
	if err := s.users.CreateWithEvent(ctx, user, event); err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
