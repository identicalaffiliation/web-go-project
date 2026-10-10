package ports

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (domain.User, error)
	Login(ctx context.Context, in LoginInput) (domain.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateToken(accessToken string) (domain.Claims, error)
}
