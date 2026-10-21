package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	authjwt "github.com/identicalaffiliation/web-go-project/auth/pkg/jwt"
)

const (
	testKID    = "kid-1"
	testIssuer = "auth-service"
)

var (
	testUser = uuid.MustParse("019a0000-0000-7000-8000-000000000001")
	testNow  = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
)

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

// rawToken собирает токен вручную, чтобы можно было подставить любой alg, kid и срок.
func rawToken(t *testing.T, method jwt.SigningMethod, key any, kid, issuer string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, jwt.MapClaims{
		"sub":  testUser.String(),
		"role": string(domain.RoleCustomer),
		"iss":  issuer,
		"iat":  testNow.Add(-time.Minute).Unix(),
		"exp":  exp.Unix(),
	})
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestVerifier_Verify(t *testing.T) {
	key := newKey(t)
	other := newKey(t)
	valid := testNow.Add(10 * time.Minute)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr error // nil — токен должен пройти
	}{
		{"valid", rawToken(t, jwt.SigningMethodRS256, key, testKID, testIssuer, valid), nil},
		{"expired", rawToken(t, jwt.SigningMethodRS256, key, testKID, testIssuer, testNow.Add(-time.Hour)), authjwt.ErrTokenExpired},
		{"wrong issuer", rawToken(t, jwt.SigningMethodRS256, key, testKID, "evil", valid), authjwt.ErrInvalidToken},
		{"unknown kid", rawToken(t, jwt.SigningMethodRS256, key, "nope", testIssuer, valid), authjwt.ErrUnknownKeyID},
		{"missing kid", rawToken(t, jwt.SigningMethodRS256, key, "", testIssuer, valid), authjwt.ErrUnknownKeyID},
		{"foreign key", rawToken(t, jwt.SigningMethodRS256, other, testKID, testIssuer, valid), authjwt.ErrInvalidToken},
		{"alg none", rawToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, testKID, testIssuer, valid), authjwt.ErrInvalidToken},
		// Классическая атака: публичный ключ используется как HMAC-секрет.
		{"alg HS256", rawToken(t, jwt.SigningMethodHS256, pubDER, testKID, testIssuer, valid), authjwt.ErrInvalidToken},
		{"garbage", "not-a-jwt", authjwt.ErrInvalidToken},
		{"empty", "", authjwt.ErrInvalidToken},
	}

	v := authjwt.NewVerifier(map[string]*rsa.PublicKey{testKID: &key.PublicKey}, testIssuer,
		authjwt.WithNow(func() time.Time { return testNow }))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.Verify(tt.token)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Verify: %v", err)
				}
				if got.UserID != testUser || got.Role != domain.RoleCustomer {
					t.Fatalf("unexpected claims: %+v", got)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want errors.Is %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifier_UnknownRole(t *testing.T) {
	key := newKey(t)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": testUser.String(), "role": "admin", "iss": testIssuer,
		"iat": testNow.Add(-time.Minute).Unix(), "exp": testNow.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = testKID
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	v := authjwt.NewVerifier(map[string]*rsa.PublicKey{testKID: &key.PublicKey}, testIssuer,
		authjwt.WithNow(func() time.Time { return testNow }))

	if _, err := v.Verify(s); !errors.Is(err, authjwt.ErrInvalidToken) || !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("got %v, want ErrInvalidToken wrapping ErrInvalidRole", err)
	}
}

// Запускать с -race: проверяет, что один Verifier безопасно делить между горутинами.
func TestVerifier_Concurrent(t *testing.T) {
	key := newKey(t)
	token := rawToken(t, jwt.SigningMethodRS256, key, testKID, testIssuer, testNow.Add(time.Hour))
	v := authjwt.NewVerifier(map[string]*rsa.PublicKey{testKID: &key.PublicKey}, testIssuer,
		authjwt.WithNow(func() time.Time { return testNow }))

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := v.Verify(token); err != nil {
				t.Errorf("Verify: %v", err)
			}
		})
	}
	wg.Wait()
}
