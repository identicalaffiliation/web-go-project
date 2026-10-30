package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"github.com/labstack/echo/v4"
)

// CreateFeedback godoc
// @Summary      Leave feedback on product
// @Description  Creates a feedback (score 1-5, optional description) on the product. Available for customers only. Requires Bearer token. X-User-Id is set by the gateway from the JWT.
// @Tags         feedbacks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        feedback  body  dto.FeedbackCreateRequest  true  "Feedback payload"
// @Success      201  {object}  dto.FeedbackResponse
// @Failure      400  "Invalid payload, score out of range or product not found"
// @Failure      401
// @Failure      403
// @Failure      500
// @Router       /catalog/feedbacks [post]
func CreateFeedback(app ports.CreateFeedbackCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var req dto.FeedbackCreateRequest
		if err := ctx.Bind(&req); err != nil {
			return echo.ErrBadRequest
		}

		req.UserID = ctx.Request().Header.Get(userIDHeader)

		response, err := app.CreateFeedback(ctx.Request().Context(), &req)
		if err != nil {
			if errors.Is(err, dto.ErrInvalidJSON) || errors.Is(err, dto.ErrInvalidData) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusCreated, response)
	}
}

// GetProductFeedbacks godoc
// @Summary      List product feedbacks (cursor pagination)
// @Description  Returns a page of product feedbacks, newest first. Use `nextCursor` from the previous response to fetch the next page. Requires Bearer token.
// @Tags         feedbacks
// @Produce      json
// @Security     BearerAuth
// @Param        productId  path   string  true   "Product ID"
// @Param        limit      query  int     false  "Page size (default 20, max 100)"
// @Param        cursor     query  string  false  "Cursor from previous response"
// @Success      200  {object}  dto.FeedbacksPage
// @Failure      400  "Invalid product id, limit or cursor"
// @Failure      401
// @Failure      403
// @Failure      500
// @Router       /catalog/{productId}/feedbacks [get]
func GetProductFeedbacks(app ports.GetProductFeedbacksCase) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		limit := int64(20)
		if v := ctx.QueryParam(limitQuery); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return echo.ErrBadRequest
			}

			limit = n
		}

		var cursor *string
		if v := ctx.QueryParam(cursorQuery); v != "" {
			cursor = &v
		}

		page, err := app.GetProductFeedbacks(ctx.Request().Context(), ctx.Param(idMux), cursor, limit)
		if err != nil {
			if errors.Is(err, dto.ErrInvalidData) {
				return echo.ErrBadRequest
			}

			return echo.ErrInternalServerError
		}

		return ctx.JSON(http.StatusOK, page)
	}
}
