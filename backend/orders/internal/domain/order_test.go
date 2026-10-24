//go:build unit

package domain

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
)

func TestOrder_ToResponseRPC(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := &Order{
		ID:         uuid.NewV7(),
		ProductID:  uuid.NewV7(),
		UserID:     uuid.NewV7(),
		Quantity:   4,
		TotalPrice: 400,
		Status:     Paid,
		CreatedAt:  now,
		UpdatedAt:  now.Add(time.Minute),
	}

	resp := order.ToResponseRPC()
	require.Equal(t, order.ID.String(), resp.GetOrderId())
	require.Equal(t, order.ProductID.String(), resp.GetProductId())
	require.Equal(t, order.UserID.String(), resp.GetUserId())
	require.Equal(t, int64(4), resp.GetQuantity())
	require.Equal(t, int64(400), resp.GetTotalPrice())
	require.Equal(t, now, resp.GetCreatedAt().AsTime())
	require.Equal(t, now.Add(time.Minute), resp.GetUpdatedAt().AsTime())
	require.Equal(t, ordersv1.OrderStatus(1), resp.GetStatus())
}

func TestOrder_ToResponseRPC_Statuses(t *testing.T) {
	for status, want := range map[Status]int32{Created: 0, Paid: 1, Failed: 2} {
		t.Run(string(status), func(t *testing.T) {
			resp := (&Order{Status: status}).ToResponseRPC()
			require.Equal(t, ordersv1.OrderStatus(want), resp.GetStatus())
		})
	}
}
