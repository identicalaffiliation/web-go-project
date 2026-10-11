package ports

//go:generate go tool mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
)

type AuthService interface {
	Register(ctx context.Context, in dto.RegisterInput) (domain.User, error)
	Login(ctx context.Context, in dto.LoginInput) (domain.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}
