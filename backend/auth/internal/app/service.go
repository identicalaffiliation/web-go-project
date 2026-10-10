package app

import (
	"fmt"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

type Service struct {
	users     ports.UserRepository
	refreshes ports.RefreshTokenRepository
	hasher    ports.PasswordHasher
	issuer    ports.AccessTokenIssuer
	verifier  ports.AccessTokenVerifier
	tokens    ports.RefreshTokenGenerator
	clock     ports.Clock
	ids       ports.IDGenerator
}

func NewService(
	users ports.UserRepository,
	refreshes ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	issuer ports.AccessTokenIssuer,
	verifier ports.AccessTokenVerifier,
	tokens ports.RefreshTokenGenerator,
	clock ports.Clock,
	ids ports.IDGenerator,
) *Service {
	return &Service{
		users:     users,
		refreshes: refreshes,
		hasher:    hasher,
		issuer:    issuer,
		verifier:  verifier,
		tokens:    tokens,
		clock:     clock,
		ids:       ids,
	}
}

func (s *Service) issueAccessAndRefresh(claims domain.Claims, now time.Time) (
	access string,
	accessExp time.Time,
	plainRefresh string,
	refreshHash string,
	refreshExp time.Time,
	err error,
) {
	access, accessExp, err = s.issuer.Issue(claims)
	if err != nil {
		return "", time.Time{}, "", "", time.Time{}, fmt.Errorf("issue access token: %w", err)
	}

	plainRefresh, refreshHash, err = s.tokens.Generate()
	if err != nil {
		return "", time.Time{}, "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshExp = now.Add(RefreshTokenTTL)
	return access, accessExp, plainRefresh, refreshHash, refreshExp, nil
}
