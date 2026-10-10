package rest

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrInvalidPassword):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, domain.ErrEmailTaken):
		return http.StatusConflict, err.Error()

	case errors.Is(err, domain.ErrInvalidCredentials):
		return http.StatusUnauthorized, "invalid credentials"

	case errors.Is(err, domain.ErrInvalidToken),
		errors.Is(err, domain.ErrTokenExpired),
		errors.Is(err, domain.ErrRefreshTokenNotFound),
		errors.Is(err, domain.ErrRefreshTokenExpired),
		errors.Is(err, domain.ErrRefreshTokenRevoked):
		return http.StatusUnauthorized, "invalid or expired token"

	default:
		return http.StatusInternalServerError, "internal server error"
	}
}
