package price

import (
	"context"
	"errors"
	"uuid"

	pricev1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/price/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	pricev1.UnimplementedPriceServiceServer
	app ports.GetItemCase
}

func NewGRPCServer(app ports.GetItemCase) *grpc.Server {
	server := grpc.NewServer()
	pricev1.RegisterPriceServiceServer(server, &GRPCServer{app: app})
	return server
}

// GetPriceByProductId - цена одного товара. Источник правды по цене - каталог.
func (s *GRPCServer) GetPriceByProductId(ctx context.Context, req *pricev1.PriceRequest) (*pricev1.PriceResponse, error) {
	response, err := s.app.GetItem(ctx, req.GetProductId())
	if err != nil {
		if errors.Is(err, dto.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, "invalid product id")
		}

		return nil, status.Error(codes.Internal, "failed to get product")
	}

	if response.Product.ProductID == uuid.Nil() {
		return nil, status.Error(codes.NotFound, "product not found")
	}

	return &pricev1.PriceResponse{
		ProductId: response.Product.ProductID.String(),
		Price:     response.Product.Price,
	}, nil
}
