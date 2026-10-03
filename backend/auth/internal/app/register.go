package app

import (
	"context"
	"fmt"

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

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	user := domain.User{
		ID:           domain.UserID(s.ids.NewID()),
		Email:        email,
		PasswordHash: hash,
		Role:         domain.RoleUser,
		CreatedAt:    s.clock.Now(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
