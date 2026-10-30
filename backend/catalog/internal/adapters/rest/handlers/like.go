package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

func LikeProduct(usecase ports.LikeItem) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		req := &struct {
			Id string `json:"id"`
		}{}

		if err := ctx.Bind(req); err != nil {
			return echo.ErrBadRequest
		}

		reqCtx := ctx.Request().Context()
		if err := usecase.LikeItem(reqCtx, req.Id); err != nil {
			if errors.Is(err, catalog.ErrInvalid) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.NoContent(http.StatusCreated)
	}
}
