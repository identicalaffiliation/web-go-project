//go:build unit

package catalog

import (
	"context"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/identicalaffiliation/web-go-project/backend/orders/gen/mocks"
	pricev1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/price/v1"
)

func TestClient_GetPrice(t *testing.T) {
	ctx := context.Background()
	productID := uuid.NewV7()
	matchRequest := mock.MatchedBy(func(req *pricev1.PriceRequest) bool {
		return req.GetProductId() == productID.String()
	})

	t.Run("returns price", func(t *testing.T) {
		api := mocks.NewMockPriceServiceClient(t)
		api.EXPECT().GetPriceByProductId(mock.Anything, matchRequest).
			Return(&pricev1.PriceResponse{ProductId: productID.String(), Price: 250}, nil).Once()

		price, err := (&Client{api: api}).GetPrice(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, int64(250), price)
	})

	t.Run("grpc error", func(t *testing.T) {
		api := mocks.NewMockPriceServiceClient(t)
		grpcErr := status.Error(codes.NotFound, "product not found")
		api.EXPECT().GetPriceByProductId(mock.Anything, matchRequest).Return(nil, grpcErr).Once()

		_, err := (&Client{api: api}).GetPrice(ctx, productID)
		require.ErrorIs(t, err, grpcErr)
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}
