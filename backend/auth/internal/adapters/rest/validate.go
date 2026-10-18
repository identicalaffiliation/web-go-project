package rest

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

func ValidateHandler(verifier ports.AccessTokenVerifier, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		token, ok := parseBearer(c.Request().Header.Get(echo.HeaderAuthorization))
		if !ok {
			log.Debug("validate: missing or malformed authorization header")
			return c.NoContent(http.StatusUnauthorized)
		}

		claims, err := verifier.Verify(token)
		if err != nil {
			// Сам токен в лог не пишем: даже просроченный он остаётся чувствительным.
			log.WithError(err).Debug("validate: token rejected")
			return c.NoContent(http.StatusUnauthorized)
		}

		h := c.Response().Header()
		h.Set("X-User-Id", claims.UserID.String())
		h.Set("X-User-Role", string(claims.Role))
		return c.NoContent(http.StatusOK)
	}
}

func parseBearer(header string) (token string, ok bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	if token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}

	return token, true
}
