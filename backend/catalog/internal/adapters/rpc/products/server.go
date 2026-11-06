package products

import (
	"context"
	"errors"

	productsv1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/products/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCServer struct {
	productsv1.UnimplementedProductsServiceServer
	app ports.GetBusinessItemsCase
}

func NewGRPCServer(app ports.GetBusinessItemsCase) *grpc.Server {
	server := grpc.NewServer()
	productsv1.RegisterProductsServiceServer(server, &GRPCServer{app: app})
	return server
}

// GetBusinessProducts - товары, выложенные бизнесом, страница с курсором (для профиля бизнеса).
func (s *GRPCServer) GetBusinessProducts(ctx context.Context, req *productsv1.GetBusinessProductsRequest) (*productsv1.GetBusinessProductsResponse, error) {
	page, err := s.app.GetBusinessItems(ctx, req.GetBusinessId(), req.Cursor, req.GetLimit())
	if err != nil {
		if errors.Is(err, dto.ErrInvalidData) {
			return nil, status.Error(codes.InvalidArgument, "invalid business id or cursor")
		}

		return nil, status.Error(codes.Internal, "failed to get business products")
	}

	result := make([]*productsv1.Product, 0, len(page.Products))
	for _, v := range page.Products {
		p := v.Product
		result = append(result, &productsv1.Product{
			ProductId:      p.ProductID.String(),
			Title:          p.Title,
			Description:    p.Description,
			Price:          p.Price,
			ImageUrl:       p.PresignedURL,
			Rating:         p.Rating,
			FeedbacksCount: p.FeedbacksCount,
			CreatedAt:      timestamppb.New(p.CreatedAt),
			UpdatedAt:      timestamppb.New(p.UpdatedAt),
		})
	}

	return &productsv1.GetBusinessProductsResponse{
		Products:   result,
		NextCursor: page.NextCursor,
	}, nil
}
