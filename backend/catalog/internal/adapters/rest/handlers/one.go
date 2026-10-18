package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/labstack/echo/v4"
)

// GetItem godoc
// @Summary      Get item by ID
// @Description  Returns a single product by its ID. Public endpoint.
// @Tags         catalog
// @Produce      json
// @Param        id  path  string  true  "Product ID"
// @Success      200  {object}  dto.ProductResponse
// @Failure      400
// @Failure      404
// @Failure      500
// @Router       /catalog/{productId} [get]
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
