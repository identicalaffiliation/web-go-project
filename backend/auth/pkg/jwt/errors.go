package jwt

import "errors"

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
	ErrUnknownKeyID = errors.New("unknown key id")
)
