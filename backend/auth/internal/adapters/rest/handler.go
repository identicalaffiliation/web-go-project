package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)



func RegisterHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req registerRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: validationMessage(err)})
		}

		user, err := svc.Register(c.Request().Context(), ports.RegisterInput{
			Email: req.Email, Password: req.Password, Role: req.Role,
		})
		if err != nil {
			return fail(c, log, err)
		}

		return c.JSON(http.StatusCreated, registerResponse{
			ID: user.ID.String(), Email: string(user.Email), Role: string(user.Role), CreatedAt: user.CreatedAt,
		})
	}
}

func LoginHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req loginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: validationMessage(err)})
		}

		pair, err := svc.Login(c.Request().Context(), ports.LoginInput{
			Email: req.Email, Password: req.Password,
		})
		if err != nil {
			return fail(c, log, err)
		}

		return c.JSON(http.StatusOK, toTokenPairResponse(pair))
	}
}

func RefreshHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req refreshRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}

		pair, err := svc.Refresh(c.Request().Context(), req.RefreshToken)
		if err != nil {
			return fail(c, log, err)
		}

		return c.JSON(http.StatusOK, toTokenPairResponse(pair))
	}
}

func LogoutHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req logoutRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}

		if err := svc.Logout(c.Request().Context(), req.RefreshToken); err != nil {
			return fail(c, log, err)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func fail(c echo.Context, log ports.Logger, err error) error {
	status, msg := mapError(err)
	if status == http.StatusInternalServerError {
		log.WithError(err).Error("auth request failed", "path", c.Path())
	}
	return c.JSON(status, errorResponse{Error: msg})
}

func toTokenPairResponse(pair domain.TokenPair) tokenPairResponse {
	return tokenPairResponse{
		AccessToken: pair.AccessToken, AccessExpiresAt: pair.AccessExpiresAt,
		RefreshToken: pair.RefreshToken, RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}
