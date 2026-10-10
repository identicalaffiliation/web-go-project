package refreshtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const TokenBytes = 32

type Generator struct{}

func New() *Generator {
	return &Generator{}
}

func (g *Generator) Generate() (plain, hash string, err error) {
	buf := make([]byte, TokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, g.Hash(plain), nil
}

func (g *Generator) Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
