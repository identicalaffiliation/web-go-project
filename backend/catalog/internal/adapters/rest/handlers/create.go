package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

// AddItemToCatalog godoc
// @Summary      Add item to catalog
// @Description  Creates a new product and returns it. Requires Bearer token.
// @Tags         catalog
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        item  body  dto.CreateProductRequest  true  "Product payload"
// @Success      201   {object}  dto.ProductResponse
// @Failure      400
// @Failure      401
// @Failure      403
// @Failure      500
// @Router       /catalog [post]
func AddItemToCatalog(app ports.AddToCatalogCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.CreateProductRequest
		if err := ctx.Bind(&req); err != nil {
			return echo.ErrBadRequest
		}

		if err := req.ValidateJSON(); err != nil {
			return echo.ErrBadRequest
		}

		reqCtx := ctx.Request().Context()
		response, err := app.AddProductToCatalog(reqCtx, &req)
		if err != nil {
			if !errors.Is(err, catalog.ErrInternal) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusCreated, response)
	}
}
