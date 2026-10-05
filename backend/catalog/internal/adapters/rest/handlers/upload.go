package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo"
)

func UploadImage(app ports.UploadImageCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		id := ctx.Param(idMux)
		format := dto.ImageFormat(ctx.QueryParam(formatQuery))

		response, err := app.UploadImage(ctx.Request().Context(), id, format)
		if err != nil {
			if errors.Is(err, dto.ErrInvalidData) {
				return echo.ErrBadRequest
			}

			if errors.Is(err, catalog.ErrNotFound) {
				return echo.ErrNotFound
			}

			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusOK, response)
	}
}
