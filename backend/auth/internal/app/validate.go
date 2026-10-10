package app

import (
	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func (s *Service) ValidateToken(accessToken string) (domain.Claims, error) {
	return s.verifier.Verify(accessToken)
}
