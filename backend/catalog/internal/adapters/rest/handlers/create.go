package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo"
)

func AddItemToCatalog(app ports.AddToCatalogCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.CreateProductRequest
		metadata := ctx.FormValue(metadataMux)
		if err := json.Unmarshal([]byte(metadata), &req); err != nil {
			return echo.ErrBadRequest
		}

		if err := req.ValidateJSON(); err != nil {
			return echo.ErrBadRequest
		}

		f, err := getFileOrNil(ctx)
		if err != nil {
			return err
		}

		req.File = f
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
