package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

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
			Subject:   string(c.UserID),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Email: string(c.Email),
		Role:  string(c.Role),
	}
}

func fromClaims(c claims) domain.Claims {
	return domain.Claims{
		UserID: domain.UserID(c.Subject),
		Email:  domain.Email(c.Email),
		Role:   domain.Role(c.Role),
	}
}
