package ports

import (
	"context"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	GetByEmail(ctx context.Context, email domain.Email) (domain.User, error)
	GetByID(ctx context.Context, id domain.UserID) (domain.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token domain.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	Rotate(ctx context.Context, oldID string, next domain.RefreshToken, now time.Time) error
	Revoke(ctx context.Context, id string, now time.Time) error
}

type PassswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

type AccessTokenIssuer interface {
	Issue(claims domain.Claims) (token string, expiresAt time.Time, err error)
}

type AccessTokenVerifier interface {
	Verify(token string) (domain.Claims, error)
}

type RefreshTokenGenerator interface {
	Generate() (plain, hash string, err error)
	Hash(plain string) string
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
