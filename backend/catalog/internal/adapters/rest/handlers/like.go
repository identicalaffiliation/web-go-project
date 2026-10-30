package handlers

import (
	"errors"
	"net/http"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/service/catalog"
	"github.com/labstack/echo/v4"
)

// LikeProduct godoc
// @Summary      Like or unlike product
// @Description  Puts (action=add) or removes (action=remove) a like on the product. Available for customers only. Requires Bearer token.
// @Tags         catalog
// @Accept       json
// @Security     BearerAuth
// @Param        like  body  dto.LikeProductRequest  true  "Product and like action"
// @Success      201
// @Failure      400
// @Failure      401
// @Failure      403
// @Failure      500
// @Router       /catalog/like [post]
func LikeProduct(usecase ports.LikeItem) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.LikeProductRequest
		if err := ctx.Bind(&req); err != nil {
			return echo.ErrBadRequest
		}

		reqCtx := ctx.Request().Context()
		if err := usecase.LikeItem(reqCtx, req.ID, req.Action); err != nil {
			if errors.Is(err, catalog.ErrInvalid) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.NoContent(http.StatusCreated)
	}
}
