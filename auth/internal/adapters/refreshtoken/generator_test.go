package refreshtoken_test

import (
	"testing"

	"github.com/identicalaffiliation/web-go-project/auth/internal/adapters/refreshtoken"
)

func TestGenerator_GenerateAndHash(t *testing.T) {
	g := refreshtoken.New()

	plain, hash, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if plain == "" || hash == "" {
		t.Fatal("plain and hash must not be empty")
	}
	if plain == hash {
		t.Fatal("plain and hash must differ")
	}
	if got := g.Hash(plain); got != hash {
		t.Errorf("Hash(plain) = %q, want %q (must match hash from Generate)", got, hash)
	}
}

func TestGenerator_Generate_IsRandom(t *testing.T) {
	g := refreshtoken.New()
	p1, _, _ := g.Generate()
	p2, _, _ := g.Generate()
	if p1 == p2 {
		t.Fatal("two generated tokens must differ")
	}
}
