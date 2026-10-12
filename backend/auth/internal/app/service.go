package app

import (
	"fmt"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type Service struct {
	users      ports.UserRepository
	refreshes  ports.RefreshTokenRepository
	hasher     ports.PasswordHasher
	issuer     ports.AccessTokenIssuer
	verifier   ports.AccessTokenVerifier
	tokens     ports.RefreshTokenGenerator
	refreshTTL time.Duration
	now        func() time.Time // подменяется в тестах
}

type Option func(*Service)


func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

// TTL access-токена здесь не нужен: его знает issuer и сам считает expiresAt.
func NewService(
	users ports.UserRepository,
	refreshes ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	issuer ports.AccessTokenIssuer,
	verifier ports.AccessTokenVerifier,
	tokens ports.RefreshTokenGenerator,
	refreshTTL time.Duration,
	opts ...Option,
) *Service {
	s := &Service{
		users:      users,
		refreshes:  refreshes,
		hasher:     hasher,
		issuer:     issuer,
		verifier:   verifier,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type issuedTokens struct {
	access       string
	accessExp    time.Time
	plainRefresh string
	refreshHash  string
	refreshExp   time.Time
}

func (s *Service) issueAccessAndRefresh(claims domain.Claims, now time.Time) (issuedTokens, error) {
	access, accessExp, err := s.issuer.Issue(claims)
	if err != nil {
		return issuedTokens{}, fmt.Errorf("issue access token: %w", err)
	}

	plainRefresh, refreshHash, err := s.tokens.Generate()
	if err != nil {
		return issuedTokens{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return issuedTokens{
		access:       access,
		accessExp:    accessExp,
		plainRefresh: plainRefresh,
		refreshHash:  refreshHash,
		refreshExp:   now.Add(s.refreshTTL),
	}, nil
}
