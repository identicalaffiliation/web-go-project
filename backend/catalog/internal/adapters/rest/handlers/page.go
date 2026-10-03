package handlers

import (
	"net/http"
	"strconv"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/labstack/echo"
)

func GetItemsPage(app ports.GetPageCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		limit, err := strconv.ParseInt(ctx.QueryParam(limitQuery), 10, 64)
		if err != nil {
			return echo.ErrBadRequest
		}

		var cursorStr *string
		if v := ctx.QueryParam(cursorQuery); v != "" {
			cursorStr = &v
		}

		page, err := app.GetItemsPage(ctx.Request().Context(), cursorStr, limit)
		if err != nil {
			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusOK, page)
	}
}
