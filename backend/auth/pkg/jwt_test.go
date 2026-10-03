package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	authjwt "github.com/identicalaffiliation/web-go-project/auth/pkg/jwt"
)

func generateKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key, &key.PublicKey
}

func TestIssueAndVerify_Success(t *testing.T) {
	priv, pub := generateKeyPair(t)
	issuer := authjwt.NewIssuer(priv, "test-kid", "auth-service", 15*time.Minute)
	verifier := authjwt.NewVerifier(pub, "auth-service")

	claims := domain.Claims{UserID: "user-1", Email: "alice@example.com", Role: domain.RoleUser}

	token, exp, err := issuer.Issue(claims)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("token must not be empty")
	}
	if !exp.After(time.Now()) {
		t.Fatalf("exp must be in the future, got %v", exp)
	}

	got, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != claims {
		t.Errorf("got %+v, want %+v", got, claims)
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	priv, pub := generateKeyPair(t)
	issuer := authjwt.NewIssuer(priv, "test-kid", "auth-service", 15*time.Minute)
	verifier := authjwt.NewVerifier(pub, "some-other-service")

	token, _, err := issuer.Issue(domain.Claims{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = verifier.Verify(token)
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	priv, pub := generateKeyPair(t)
	// TTL в прошлом — токен просрочен сразу после выпуска.
	issuer := authjwt.NewIssuer(priv, "test-kid", "auth-service", -time.Minute)
	verifier := authjwt.NewVerifier(pub, "auth-service")

	token, _, err := issuer.Issue(domain.Claims{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = verifier.Verify(token)
	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

func TestVerify_WrongKey(t *testing.T) {
	priv1, _ := generateKeyPair(t)
	_, pub2 := generateKeyPair(t)

	issuer := authjwt.NewIssuer(priv1, "test-kid", "auth-service", 15*time.Minute)
	verifier := authjwt.NewVerifier(pub2, "auth-service") // чужой публичный ключ

	token, _, err := issuer.Issue(domain.Claims{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = verifier.Verify(token)
	if !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken (signature mismatch), got %v", err)
	}
}
