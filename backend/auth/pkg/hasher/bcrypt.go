package hasher

import (
	"fmt"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type Hasher struct{}

func New() *Hasher {
	return &Hasher{}
}

func (h *Hasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt generate: %w", err)
	}
	return string(hash), nil
}

// Compare возвращает одну и ту же ошибку и для неверного пароля, и для битого хеша:
// вызывающему незачем различать эти случаи, а наружу такая разница не должна утекать.
func (h *Hasher) Compare(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return domain.ErrInvalidCredentials
	}
	return nil
}
