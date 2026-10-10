package domain

import (
	"fmt"
	"unicode/utf8"
)

const (
	MinPasswordLen   = 8
	MaxPasswordBytes = 72
)

func ValidatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < MinPasswordLen {
		return fmt.Errorf("%w: too short", ErrInvalidPassword)
	}

	if len(plain) > MaxPasswordBytes {
		return fmt.Errorf("%w: too long", ErrInvalidPassword)
	}

	return nil
}
