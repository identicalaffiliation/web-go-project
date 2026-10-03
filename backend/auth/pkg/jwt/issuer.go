package jwt

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type Issuer struct {
	privateKey *rsa.PrivateKey
	kid        string
	issuer     string
	ttl        time.Duration
	now        func() time.Time // подменяется в тестах
}

func NewIssuer(privateKey *rsa.PrivateKey, kid, issuerName string, ttl time.Duration) *Issuer {
	return &Issuer{
		privateKey: privateKey,
		kid:        kid,
		issuer:     issuerName,
		ttl:        ttl,
		now:        time.Now,
	}
}

func (i *Issuer) Issue(c domain.Claims) (string, time.Time, error) {
	now := i.now()
	cl := toClaims(c, i.issuer, now, i.ttl)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, cl)
	token.Header["kid"] = i.kid // чтобы верификатор мог выбрать нужный ключ при ротации

	signed, err := token.SignedString(i.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, cl.ExpiresAt.Time, nil
}

var _ ports.AccessTokenIssuer = (*Issuer)(nil)
