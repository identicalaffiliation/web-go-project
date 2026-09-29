package domain

import (
	"net/mail"
	"strings"
	"time"
)

const maxEmaiLen = 254

type UserID string

type Role string

type Email string

const (
	RoleUser     Role = "user"
	RoleBusiness Role = "business"
)

func (r Role) Valid() bool {
	return r == RoleUser || r == RoleBusiness
}

func NewEmail(raw string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || len(normalized) > maxEmaiLen {
		return "", ErrInvalidEmail
	}

	addr, err := mail.ParseAddress(normalized)
	if err != nil || addr.Address != normalized {
		return "", ErrInvalidEmail
	}

	return Email(normalized), nil
}

type User struct {
	ID           UserID
	Email        Email
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}
