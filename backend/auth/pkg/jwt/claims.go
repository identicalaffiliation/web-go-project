package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

func toClaims(c domain.Claims, issuer string, now time.Time, ttl time.Duration) claims {
	return claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.UserID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Email: string(c.Email),
		Role:  string(c.Role),
	}
}

func fromClaims(c claims) (domain.Claims, error) {
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return domain.Claims{}, fmt.Errorf("parse subject: %w", err)
	}

	// Роль уходит в заголовок X-User-Role, поэтому неизвестное значение отсекаем здесь.
	if !domain.Role(c.Role).Valid() {
		return domain.Claims{}, fmt.Errorf("%w: %q", domain.ErrInvalidRole, c.Role)
	}

	return domain.Claims{
		UserID: userID,
		Email:  domain.Email(c.Email),
		Role:   domain.Role(c.Role),
	}, nil
}
