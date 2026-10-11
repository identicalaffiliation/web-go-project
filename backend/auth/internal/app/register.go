package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func (s *Service) Register(ctx context.Context, in ports.RegisterInput) (domain.User, error) {
	email, err := domain.NewEmail(in.Email)
	if err != nil {
		return domain.User{}, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return domain.User{}, err
	}
	// Дефолта нет: пустая или неизвестная роль (в т.ч. admin) отвергается,
	// чтобы клиент не мог выдать себе привилегии при регистрации.
	role := domain.Role(in.Role)
	if !role.Valid() {
		return domain.User{}, domain.ErrInvalidRole
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.User{}, fmt.Errorf("generate user id: %w", err)
	}

	user := domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    s.now(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
