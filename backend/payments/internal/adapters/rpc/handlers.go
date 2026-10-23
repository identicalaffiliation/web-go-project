package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/service/balance"
)

type Service interface {
	GetBalance(ctx context.Context, id string) (*balancev1.GetBalanceResponse, error)
}

type Handler struct {
	balancev1.UnimplementedPaymentsServiceServer
	service Service
}

func NewRPChandler(svc Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) GetBalanceByUser(ctx context.Context, req *balancev1.GetBalanceRequest) (*balancev1.GetBalanceResponse, error) {
	response, err := h.service.GetBalance(ctx, req.GetUserId())
	if err != nil {
		switch {
		case errors.Is(err, balance.ErrInvalidData):
			return nil, status.Error(codes.InvalidArgument, "invalid user_id")
		case errors.Is(err, domain.ErrAccountNotFound):
			return nil, status.Error(codes.NotFound, "account not found")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return response, nil
}
