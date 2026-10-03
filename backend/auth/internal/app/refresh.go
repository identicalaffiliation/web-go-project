package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func (s *Service) Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error) {
	hash := s.tokens.Hash(refreshToken)

	stored, err := s.refreshes.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return domain.TokenPair{}, domain.ErrInvalidToken
		}
		return domain.TokenPair{}, fmt.Errorf("get refresh token: %w", err)
	}

	now := s.clock.Now()
	if err := stored.CheckActive(now); err != nil {
		return domain.TokenPair{}, err
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return domain.TokenPair{}, fmt.Errorf("get user: %w", err)
	}

	claims := domain.Claims{UserID: user.ID, Email: user.Email, Role: user.Role}
	access, accessExp, plainRefresh, newHash, refreshExp, err := s.issueAccessAndRefresh(claims, now)
	if err != nil {
		return domain.TokenPair{}, err
	}

	next := domain.RefreshToken{
		ID:        s.ids.NewID(),
		UserID:    user.ID,
		TokenHash: newHash,
		ExpiresAt: refreshExp,
		CreatedAt: now,
	}
	if err := s.refreshes.Rotate(ctx, stored.ID, next, now); err != nil {
		return domain.TokenPair{}, fmt.Errorf("rotate refresh token: %w", err)
	}

	return domain.TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: refreshExp,
	}, nil
}
