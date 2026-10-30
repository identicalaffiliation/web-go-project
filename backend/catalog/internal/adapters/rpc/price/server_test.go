//go:build unit

package price

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	pricev1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/price/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPriceByProductId(t *testing.T) {
	ctx := context.Background()
	productID := uuid.New()

	t.Run("ok", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(ctx, productID.String()).Return(&dto.ProductResponse{
			Product: dto.Product{ProductID: productID, Price: 1500},
		}, nil)

		s := &GRPCServer{app: app}
		resp, err := s.GetPriceByProductId(ctx, &pricev1.PriceRequest{ProductId: productID.String()})
		require.NoError(t, err)
		require.Equal(t, productID.String(), resp.GetProductId())
		require.Equal(t, int64(1500), resp.GetPrice())
	})

	t.Run("invalid id", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(ctx, "bad").Return(nil, dto.ErrInvalidData)

		s := &GRPCServer{app: app}
		_, err := s.GetPriceByProductId(ctx, &pricev1.PriceRequest{ProductId: "bad"})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("not found", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(ctx, productID.String()).Return(&dto.ProductResponse{}, nil)

		s := &GRPCServer{app: app}
		_, err := s.GetPriceByProductId(ctx, &pricev1.PriceRequest{ProductId: productID.String()})
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("internal", func(t *testing.T) {
		app := mocks.NewMockGetItemCase(gomock.NewController(t))
		app.EXPECT().GetItem(ctx, productID.String()).Return(nil, errors.New("db down"))

		s := &GRPCServer{app: app}
		_, err := s.GetPriceByProductId(ctx, &pricev1.PriceRequest{ProductId: productID.String()})
		require.Equal(t, codes.Internal, status.Code(err))
	})
}
