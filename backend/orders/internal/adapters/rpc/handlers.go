package rpc

import (
	"context"
	"errors"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/service/orders"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service interface {
	GetUserOrders(ctx context.Context, id string) (*ordersv1.UserOrdersResponse, error)
}

type Handler struct {
	ordersv1.UnimplementedOrdersServiceServer
	service Service
}

func NewRPChandler(svc Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) GetUserOrders(ctx context.Context, req *ordersv1.UserOrdersRequest) (*ordersv1.UserOrdersResponse, error) {
	response, err := h.service.GetUserOrders(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, orders.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, "invalid user_id")
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	return response, nil
}
