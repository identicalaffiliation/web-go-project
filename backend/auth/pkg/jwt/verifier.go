package jwt

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

// leeway сглаживает расхождение часов между auth и тем, кто проверяет токен.
const leeway = 5 * time.Second

// Verifier безопасен для конкурентного использования: после конструктора и keys,
// и parser только читаются.
type Verifier struct {
	keys   map[string]*rsa.PublicKey
	parser *jwt.Parser
	now    func() time.Time // подменяется в тестах
}

type VerifierOption func(*Verifier)

func WithNow(now func() time.Time) VerifierOption {
	return func(v *Verifier) {
		v.now = now
	}
}

// NewVerifier копирует keys: если вызывающий код потом изменит свою карту,
// это не приведёт к гонке с параллельными Verify.
func NewVerifier(keys map[string]*rsa.PublicKey, expectedIssuer string, opts ...VerifierOption) *Verifier {
	v := &Verifier{
		keys: maps.Clone(keys),
		now:  time.Now,
	}
	for _, opt := range opts {
		opt(v)
	}

	// Parser создаём один раз, а не на каждый запрос. WithTimeFunc получает
	// замыкание, а не v.now напрямую, чтобы учесть опции, применённые выше.
	v.parser = jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(expectedIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
		jwt.WithTimeFunc(v.now),
	)
	return v
}

func (v *Verifier) Verify(tokenString string) (domain.Claims, error) {
	var cl claims

	token, err := v.parser.ParseWithClaims(tokenString, &cl, v.keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return domain.Claims{}, fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return domain.Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return domain.Claims{}, ErrInvalidToken
	}

	out, err := fromClaims(cl)
	if err != nil {
		return domain.Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return out, nil
}

func (v *Verifier) keyFunc(t *jwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	key, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKeyID, kid)
	}
	return key, nil
}
