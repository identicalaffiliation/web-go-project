package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

// DeleteItem godoc
// @Summary      Delete own product
// @Description  Removes the product from the catalog: it disappears from the list, the card and prices for orders. Available for the business that owns the product. Requires Bearer token. X-User-Id (product owner) is set by the gateway from the JWT.
// @Tags         catalog
// @Security     BearerAuth
// @Param        productId  path  string  true  "Product ID"
// @Success      204
// @Failure      400  "Invalid product id"
// @Failure      401
// @Failure      403
// @Failure      404  "Product not found, already deleted or owned by another business"
// @Failure      500
// @Router       /catalog/{productId} [delete]
func DeleteItem(app ports.DeleteItemCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		businessID := ctx.Request().Header.Get(userIDHeader)

		if err := app.DeleteProduct(ctx.Request().Context(), businessID, ctx.Param(idMux)); err != nil {
			if errors.Is(err, dto.ErrInvalidData) {
				return echo.ErrBadRequest
			}

			if errors.Is(err, catalog.ErrNotFound) {
				return echo.ErrNotFound
			}

			return echo.ErrInternalServerError
		}

		return ctx.NoContent(http.StatusNoContent)
	}
}
