package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/dto"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// RegisterHandler godoc
// @Summary      Register a new user
// @Description  Creates a user with the customer role (business is granted later via the Profile event). Password: 8-72 bytes, not blank.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      registerRequest  true  "Registration payload"
// @Success      201   {object}  registerResponse
// @Failure      400   {object}  errorResponse  "Invalid body, email or password"
// @Failure      409   {object}  errorResponse  "Email already taken"
// @Failure      500   {object}  errorResponse
// @Router       /register [post]
func RegisterHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req registerRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: validationMessage(err)})
		}

		user, err := svc.Register(c.Request().Context(), dto.RegisterInput{
			Email: req.Email, Password: req.Password,
		})
		if err != nil {
			return fail(c, log, err)
		}

		return c.JSON(http.StatusCreated, registerResponse{
			ID: user.ID.String(), Email: string(user.Email), Role: string(user.Role), CreatedAt: user.CreatedAt,
		})
	}
}

// LoginHandler godoc
// @Summary      Log in
// @Description  Checks credentials and returns an access JWT (use it as "Authorization: Bearer <token>") and a refresh token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest  true  "Credentials"
// @Success      200   {object}  tokenPairResponse
// @Failure      400   {object}  errorResponse  "Invalid body, email or password missing"
// @Failure      401   {object}  errorResponse  "Invalid credentials"
// @Failure      500   {object}  errorResponse
// @Router       /login [post]
func LoginHandler(svc ports.AuthService, log ports.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req loginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, errorResponse{Error: validationMessage(err)})
		}

		pair, err := svc.Login(c.Request().Context(), dto.LoginInput{
			Email: req.Email, Password: req.Password,
		})
		if err != nil {
			return fail(c, log, err)
		}

		return c.JSON(http.StatusOK, toTokenPairResponse(pair))
	}
}

// RefreshHandler godoc
// @Summary      Refresh tokens
// @Description  Exchanges a valid refresh token for a new pair. The old refresh token is revoked (rotation).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      refreshRequest  true  "Refresh token"
// @Success      200   {object}  tokenPairResponse
// @Failure      400   {object}  errorResponse  "Invalid body"
// @Failure      401   {object}  errorResponse  "Refresh token unknown, expired or revoked"
// @Failure      500   {object}  errorResponse
// @Router       /refresh [post]
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

// LogoutHandler godoc
// @Summary      Log out
// @Description  Revokes the refresh token. Idempotent: an unknown token also gives 204.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  logoutRequest  true  "Refresh token"
// @Success      204   "Token revoked"
// @Failure      400   {object}  errorResponse  "Invalid body"
// @Failure      500   {object}  errorResponse
// @Router       /logout [post]
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
