package handlers

import (
	"net/http"
	"strconv"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/labstack/echo/v4"
)

// GetItemsPage godoc
// @Summary      List catalog items (cursor pagination)
// @Description  Returns a page of catalog items. Public endpoint. Use `cursor` from the previous response to fetch the next page.
// @Tags         catalog
// @Produce      json
// @Param        limit   query  int     false  "Page size (default 50)"
// @Param        cursor  query  string  false  "Opaque cursor from previous response"
// @Success      200
// @Failure      400
// @Failure      500
// @Router       /catalog [get]
func GetItemsPage(app ports.GetPageCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		limit := int64(50)
		if v := ctx.QueryParam(limitQuery); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return echo.ErrBadRequest
			}

			limit = n
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
