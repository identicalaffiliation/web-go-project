package domain

import "errors"

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidRole     = errors.New("invalid role")
	ErrEmailTaken      = errors.New("email already taken")

	// ErrInvalidEvent — входящее событие Kafka битое; повтор его не исправит.
	ErrInvalidEvent = errors.New("invalid event")

	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrRefreshTokenExpired = errors.New("refresh token expired")
	ErrRefreshTokenRevoked = errors.New("refresh token revoked")
)
