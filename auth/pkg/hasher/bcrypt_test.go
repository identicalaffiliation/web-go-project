package hasher_test

import (
	"errors"
	"testing"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/pkg/hasher"
)

func TestHasher_HashAndCompare(t *testing.T) {
	h := hasher.New(4) // низкая cost — тест быстрый, тут проверяем не стойкость, а поведение

	hash, err := h.Hash("password1")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "password1" {
		t.Fatal("hash must not equal plain password")
	}
	if err := h.Compare(hash, "password1"); err != nil {
		t.Fatalf("Compare with correct password: %v", err)
	}
}

func TestHasher_Compare_WrongPassword(t *testing.T) {
	h := hasher.New(4)

	hash, err := h.Hash("password1")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	err = h.Compare(hash, "wrong-password")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}
