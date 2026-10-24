//go:build unit

package dto

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

func validRequest() CreateRequest {
	return CreateRequest{
		ProductID:   uuid.NewV7(),
		OperationID: uuid.NewV7(),
		UserID:      uuid.NewV7(),
		Quantity:    5,
	}
}

func TestCreateRequest_Validate(t *testing.T) {
	cases := []struct {
		name    string
		modify  func(r *CreateRequest)
		wantErr bool
	}{
		{name: "valid", modify: func(*CreateRequest) {}},
		{name: "max quantity", modify: func(r *CreateRequest) { r.Quantity = 99 }},
		{name: "nil product id", modify: func(r *CreateRequest) { r.ProductID = uuid.Nil() }, wantErr: true},
		{name: "nil operation id", modify: func(r *CreateRequest) { r.OperationID = uuid.Nil() }, wantErr: true},
		{name: "nil user id", modify: func(r *CreateRequest) { r.UserID = uuid.Nil() }, wantErr: true},
		{name: "zero quantity", modify: func(r *CreateRequest) { r.Quantity = 0 }, wantErr: true},
		{name: "negative quantity", modify: func(r *CreateRequest) { r.Quantity = -1 }, wantErr: true},
		{name: "quantity too big", modify: func(r *CreateRequest) { r.Quantity = 100 }, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validRequest()
			tc.modify(&req)

			err := req.Validate()
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidData)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestCreateRequest_ToDomain(t *testing.T) {
	req := validRequest()

	order := req.ToDomain()
	require.NotEqual(t, uuid.Nil(), order.ID)
	require.Equal(t, req.ProductID, order.ProductID)
	require.Equal(t, req.UserID, order.UserID)
	require.Equal(t, req.OperationID, order.OperationID)
	require.Equal(t, req.Quantity, order.Quantity)

	// каждый запрос - новый заказ
	require.NotEqual(t, order.ID, req.ToDomain().ID)
}

func TestNewOrderResponse(t *testing.T) {
	order := &domain.Order{ID: uuid.NewV7(), Status: domain.Created}

	require.Equal(t, *order, NewOrderResponse(order).Order)
}
