package hasher

import (
	"errors"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

const DefaultCost = bcrypt.DefaultCost

type Hasher struct {
	cost int
}

func New(cost int) *Hasher {
	if cost <= 0 {
		cost = DefaultCost
	}
	return &Hasher{cost: cost}
}

func (h *Hasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt generate: %w", err)
	}
	return string(hash), nil
}

func (h *Hasher) Compare(hash, plain string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return domain.ErrInvalidCredentials
		}
		return fmt.Errorf("bcrypt compare: %w", err)
	}
	return nil
}

var _ ports.PasswordHasher = (*Hasher)(nil)
