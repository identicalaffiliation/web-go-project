package rest

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type Handler struct {
	service ports.AuthService
	logger  *slog.Logger
}

func NewHandler(service ports.AuthService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Register монтирует маршруты под переданной группой, например e.Group("/api/v1/auth").
func (h *Handler) Register(g *echo.Group) {
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
	g.POST("/validate", h.validate)
}

func (h *Handler) register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
	}

	user, err := h.service.Register(c.Request().Context(), ports.RegisterInput{
		Email: req.Email, Password: req.Password,
	})
	if err != nil {
		return h.fail(c, err)
	}

	return c.JSON(http.StatusCreated, registerResponse{
		ID: string(user.ID), Email: string(user.Email), Role: string(user.Role), CreatedAt: user.CreatedAt,
	})
}

func (h *Handler) login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
	}

	pair, err := h.service.Login(c.Request().Context(), ports.LoginInput{
		Email: req.Email, Password: req.Password,
	})
	if err != nil {
		return h.fail(c, err)
	}

	return c.JSON(http.StatusOK, toTokenPairResponse(pair))
}

func (h *Handler) refresh(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
	}

	pair, err := h.service.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return h.fail(c, err)
	}

	return c.JSON(http.StatusOK, toTokenPairResponse(pair))
}

func (h *Handler) logout(c echo.Context) error {
	var req logoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
	}

	if err := h.service.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return h.fail(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) validate(c echo.Context) error {
	var req validateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid request body"})
	}

	claims, err := h.service.ValidateToken(c.Request().Context(), req.AccessToken)
	if err != nil {
		return h.fail(c, err)
	}

	return c.JSON(http.StatusOK, claimsResponse{
		UserID: string(claims.UserID), Email: string(claims.Email), Role: string(claims.Role),
	})
}

func (h *Handler) fail(c echo.Context, err error) error {
	status, msg := mapError(err)
	if status == http.StatusInternalServerError {
		h.logger.Error("auth request failed", "error", err, "path", c.Path())
	}
	return c.JSON(status, errorResponse{Error: msg})
}

func toTokenPairResponse(pair domain.TokenPair) tokenPairResponse {
	return tokenPairResponse{
		AccessToken: pair.AccessToken, AccessExpiresAt: pair.AccessExpiresAt,
		RefreshToken: pair.RefreshToken, RefreshExpiresAt: pair.RefreshExpiresAt,
	}
}
