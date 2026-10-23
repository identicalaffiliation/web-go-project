//go:build unit

package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/orders/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
)

func doCreate(t *testing.T, svc Service, userID, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	e.POST("/orders", CreateOrder(svc))

	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if userID != "" {
		req.Header.Set(UserIdHeaderPattern, userID)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func validBody(productID, operationID uuid.UUID, quantity int) string {
	b, _ := json.Marshal(map[string]any{
		"productId":   productID.String(),
		"operationId": operationID.String(),
		"quantity":    quantity,
	})

	return string(b)
}

func TestCreateOrder(t *testing.T) {
	userID := uuid.NewV7()
	productID := uuid.NewV7()
	operationID := uuid.NewV7()

	matchRequest := mock.MatchedBy(func(req *dto.CreateRequest) bool {
		return req.UserID == userID && req.ProductID == productID && req.OperationID == operationID && req.Quantity == 2
	})

	t.Run("created", func(t *testing.T) {
		svc := mocks.NewMockCreateOrderService(t)
		order := domain.Order{ID: uuid.NewV7(), UserID: userID, ProductID: productID, Quantity: 2, Status: domain.Created}
		svc.EXPECT().CreateOrder(mock.Anything, matchRequest).Return(&dto.OrderResponse{Order: order}, nil).Once()

		rec := doCreate(t, svc, userID.String(), validBody(productID, operationID, 2))
		require.Equal(t, http.StatusCreated, rec.Code)

		var resp dto.OrderResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, order.ID, resp.Order.ID)
		require.Equal(t, domain.Created, resp.Order.Status)
	})

	t.Run("duplicate operation id is conflict", func(t *testing.T) {
		svc := mocks.NewMockCreateOrderService(t)
		svc.EXPECT().CreateOrder(mock.Anything, matchRequest).Return(nil, orders.ErrOrderAlreadyExists).Once()

		rec := doCreate(t, svc, userID.String(), validBody(productID, operationID, 2))
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("invalid data from service", func(t *testing.T) {
		svc := mocks.NewMockCreateOrderService(t)
		svc.EXPECT().CreateOrder(mock.Anything, matchRequest).Return(nil, orders.ErrInvalidData).Once()

		rec := doCreate(t, svc, userID.String(), validBody(productID, operationID, 2))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("internal error", func(t *testing.T) {
		svc := mocks.NewMockCreateOrderService(t)
		svc.EXPECT().CreateOrder(mock.Anything, matchRequest).Return(nil, errors.New("boom")).Once()

		rec := doCreate(t, svc, userID.String(), validBody(productID, operationID, 2))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	badRequests := []struct {
		name   string
		userID string
		body   string
	}{
		{name: "broken json", userID: userID.String(), body: `{"productId":`},
		{name: "no user id header", userID: "", body: validBody(productID, operationID, 2)},
		{name: "invalid user id header", userID: "not-uuid", body: validBody(productID, operationID, 2)},
		{name: "zero quantity", userID: userID.String(), body: validBody(productID, operationID, 0)},
		{name: "quantity too big", userID: userID.String(), body: validBody(productID, operationID, 100)},
		{name: "nil product id", userID: userID.String(), body: validBody(uuid.Nil(), operationID, 2)},
		{name: "nil operation id", userID: userID.String(), body: validBody(productID, uuid.Nil(), 2)},
	}

	for _, tc := range badRequests {
		t.Run(tc.name, func(t *testing.T) {
			// сервис не должен вызываться - мок упадет на неожиданном вызове
			svc := mocks.NewMockCreateOrderService(t)

			rec := doCreate(t, svc, tc.userID, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
