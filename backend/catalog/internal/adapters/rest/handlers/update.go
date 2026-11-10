package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

// UpdateItem godoc
// @Summary      Update own product
// @Description  Changes title, description and/or price of the product (only passed fields, at least one). Available for the business that owns the product. Requires Bearer token. X-User-Id (product owner) is set by the gateway from the JWT.
// @Tags         catalog
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        productId  path  string                    true  "Product ID"
// @Param        item       body  dto.UpdateProductRequest  true  "Fields to change"
// @Success      200  {object}  dto.ProductResponse
// @Failure      400  "Invalid id or payload, no fields to change"
// @Failure      401
// @Failure      403
// @Failure      404  "Product not found, deleted or owned by another business"
// @Failure      500
// @Router       /catalog/{productId} [patch]
func UpdateItem(app ports.UpdateItemCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.UpdateProductRequest
		if err := ctx.Bind(&req); err != nil {
			return echo.ErrBadRequest
		}

		req.ProductID = ctx.Param(idMux)
		req.BusinessID = ctx.Request().Header.Get(userIDHeader)

		if err := req.ValidateJSON(); err != nil {
			return echo.ErrBadRequest
		}

		response, err := app.UpdateProduct(ctx.Request().Context(), &req)
		if err != nil {
			if errors.Is(err, dto.ErrInvalidJSON) || errors.Is(err, dto.ErrInvalidData) {
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
