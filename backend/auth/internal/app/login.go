package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
)

func (s *Service) Login(ctx context.Context, in dto.LoginInput) (domain.TokenPair, error) {
	email, err := domain.NewEmail(in.Email)
	if err != nil {
		return domain.TokenPair{}, domain.ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.TokenPair{}, domain.ErrInvalidCredentials
		}
		return domain.TokenPair{}, fmt.Errorf("get user: %w", err)
	}

	if err := s.hasher.Compare(user.PasswordHash, in.Password); err != nil {
		return domain.TokenPair{}, domain.ErrInvalidCredentials
	}

	now := s.now()
	claims := domain.Claims{UserID: user.ID, Email: user.Email, Role: user.Role}
	t, err := s.issueAccessAndRefresh(claims, now)
	if err != nil {
		return domain.TokenPair{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("generate refresh token id: %w", err)
	}

	rt := domain.RefreshToken{
		ID:        id,
		UserID:    user.ID,
		TokenHash: t.refreshHash,
		ExpiresAt: t.refreshExp,
		CreatedAt: now,
	}
	if err := s.refreshes.Create(ctx, rt); err != nil {
		return domain.TokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}

	return domain.TokenPair{
		AccessToken:      t.access,
		AccessExpiresAt:  t.accessExp,
		RefreshToken:     t.plainRefresh,
		RefreshExpiresAt: t.refreshExp,
	}, nil
}
