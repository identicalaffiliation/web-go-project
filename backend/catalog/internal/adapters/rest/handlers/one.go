package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/labstack/echo"
)

func GetItem(app ports.GetItemCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		id := ctx.Param(idMux)
		reqCtx := ctx.Request().Context()

		response, err := app.GetItem(reqCtx, id)
		if err != nil {
			if errors.Is(err, dto.ErrInvalidData) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusOK, response)
	}
}
