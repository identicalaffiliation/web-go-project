package app

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func (s *Service) ValidateToken(_ context.Context, accessToken string) (domain.Claims, error) {
	return s.verifier.Verify(accessToken)
}
