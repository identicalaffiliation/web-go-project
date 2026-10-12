package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLen   = 8
	MaxPasswordBytes = 72
)

func ValidatePassword(plain string) error {
	// Восемь пробелов проходят проверку длины, но паролем не являются.
	if strings.TrimSpace(plain) == "" {
		return fmt.Errorf("%w: blank", ErrInvalidPassword)
	}

	if utf8.RuneCountInString(plain) < MinPasswordLen {
		return fmt.Errorf("%w: too short", ErrInvalidPassword)
	}

	if len(plain) > MaxPasswordBytes {
		return fmt.Errorf("%w: too long", ErrInvalidPassword)
	}

	return nil
}
