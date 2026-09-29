package domain

import "errors"

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password") // пароль не прошёл правила при регистрации
	ErrEmailTaken      = errors.New("email already taken")
	ErrUserNotFound    = errors.New("user not found")

	// Одна ошибка и для «нет такого email», и для «неверный пароль»:
	// по ответу нельзя выяснить, зарегистрирован ли адрес.
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")

	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
)

