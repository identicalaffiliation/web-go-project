package controller

import (
	"context"
	"errors"
	"net/http"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
	"github.com/labstack/echo/v4"
)

const (
	UserIdHeaderPattern = "X-User-Id"
)

type Service interface {
	CreateOrder(ctx context.Context, req *dto.CreateRequest) (*dto.OrderResponse, error)
}

// CreateOrder godoc
// @Summary      Create order
// @Description  Creates an order with status "created" and starts the order saga asynchronously.
// @Description  Final status (paid/failed) is available via GetUserOrders.
// @Description  operationId is an idempotency key: a repeated request with the same key is rejected with 409.
// @Description  X-User-Id and X-User-Role headers are set by the gateway from the JWT.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        X-User-Id    header  string             true  "User ID (uuid)"
// @Param        X-User-Role  header  string             true  "User role"  Enums(customer)
// @Param        order        body    dto.CreateRequest  true  "Order payload"
// @Success      201  {object}  dto.OrderResponse
// @Failure      400  "Invalid payload or X-User-Id"
// @Failure      401  "No X-User-Role"
// @Failure      403  "Role is not allowed"
// @Failure      409  "Order with this operationId already exists"
// @Failure      500
// @Router       /orders [post]
func CreateOrder(svc Service) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.CreateRequest
		if err := ctx.Bind(&req); err != nil {
			return echo.ErrBadRequest
		}

		id, err := uuid.Parse(ctx.Request().Header.Get(UserIdHeaderPattern))
		if err != nil {
			return echo.ErrBadRequest
		}

		req.UserID = id
		if err := req.Validate(); err != nil {
			return echo.ErrBadRequest
		}

		reqCtx := ctx.Request().Context()
		resp, err := svc.CreateOrder(reqCtx, &req)
		switch {
		case errors.Is(err, orders.ErrOrderAlreadyExists):
			return echo.ErrConflict
		case errors.Is(err, orders.ErrInvalidData):
			return echo.ErrBadRequest
		case err != nil:
			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusCreated, resp)
	}
}
