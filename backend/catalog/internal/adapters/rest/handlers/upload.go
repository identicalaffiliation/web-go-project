package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

// UploadImage godoc
// @Summary      Upload product image
// @Description  Generates a pre-signed URL (or uploads an image) for the product. Requires Bearer token.
// @Tags         catalog
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path   string  true   "Product ID"
// @Param        format  query  string  false  "Image format"  Enums(jpeg, png)
// @Success      200  {object}  dto.ProductResponse
// @Failure      400
// @Failure      401
// @Failure      403
// @Failure      404
// @Failure      500
// @Router       /catalog/{productId}/image [post]
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
