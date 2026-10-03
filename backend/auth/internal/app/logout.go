package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	hash := s.tokens.Hash(refreshToken)

	stored, err := s.refreshes.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil
		}
		return fmt.Errorf("get refresh token: %w", err)
	}

	if err := s.refreshes.Revoke(ctx, stored.ID, s.clock.Now()); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
