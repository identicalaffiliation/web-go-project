package jwt

import (
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

type Verifier struct {
	publicKey      *rsa.PublicKey
	expectedIssuer string
}

func NewVerifier(publicKey *rsa.PublicKey, expectedIssuer string) *Verifier {
	return &Verifier{publicKey: publicKey, expectedIssuer: expectedIssuer}
}

var errUnexpectedSigningMethod = errors.New("unexpected signing method")

func (v *Verifier) Verify(tokenString string) (domain.Claims, error) {
	var cl claims

	token, err := jwt.ParseWithClaims(tokenString, &cl, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: %v", errUnexpectedSigningMethod, t.Header["alg"])
		}
		return v.publicKey, nil
	}, jwt.WithIssuer(v.expectedIssuer), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return domain.Claims{}, domain.ErrTokenExpired
		}
		return domain.Claims{}, fmt.Errorf("%w: %v", domain.ErrInvalidToken, err)
	}
	if !token.Valid {
		return domain.Claims{}, domain.ErrInvalidToken
	}

	return fromClaims(cl), nil
}
