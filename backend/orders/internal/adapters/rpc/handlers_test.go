//go:build unit

package rpc

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/identicalaffiliation/web-go-project/backend/orders/gen/mocks"
	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
)

func TestHandler_GetUserOrders(t *testing.T) {
	ctx := context.Background()
	userID := uuid.NewV7().String()
	req := &ordersv1.UserOrdersRequest{UserId: userID}

	t.Run("returns orders", func(t *testing.T) {
		svc := mocks.NewMockUserOrdersService(t)
		want := &ordersv1.UserOrdersResponse{Orders: []*ordersv1.UserOrderResponse{{UserId: userID}}}
		svc.EXPECT().GetUserOrders(mock.Anything, userID).Return(want, nil).Once()

		resp, err := NewRPChandler(svc).GetUserOrders(ctx, req)
		require.NoError(t, err)
		require.Equal(t, want, resp)
	})

	t.Run("invalid user id", func(t *testing.T) {
		svc := mocks.NewMockUserOrdersService(t)
		svc.EXPECT().GetUserOrders(mock.Anything, userID).Return(nil, orders.ErrInvalidData).Once()

		_, err := NewRPChandler(svc).GetUserOrders(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("internal error", func(t *testing.T) {
		svc := mocks.NewMockUserOrdersService(t)
		svc.EXPECT().GetUserOrders(mock.Anything, userID).Return(nil, errors.New("db down")).Once()

		_, err := NewRPChandler(svc).GetUserOrders(ctx, req)
		require.Equal(t, codes.Internal, status.Code(err))
	})
}
