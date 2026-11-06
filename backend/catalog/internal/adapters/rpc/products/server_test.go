//go:build unit

package products

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	productsv1 "github.com/identicalaffiliation/web-go-project/backend/catalog/gen/pb/products/v1"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetBusinessProducts(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7().String()

	t.Run("ok - mapped to proto", func(t *testing.T) {
		app := mocks.NewMockGetBusinessItemsCase(gomock.NewController(t))
		createdAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(time.Hour)
		p := dto.Product{
			ProductID:      uuid.NewV7(),
			Title:          "title",
			Description:    new("desc"),
			Price:          1500,
			PresignedURL:   new("http://url"),
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
			Rating:         4.5,
			FeedbacksCount: 2,
		}
		cursor, next := uuid.NewV7().String(), uuid.NewV7().String()

		app.EXPECT().GetBusinessItems(ctx, businessID, &cursor, int64(10)).Return(&dto.Page{
			Products:   []dto.ProductResponse{{Product: p}, {Product: dto.Product{ProductID: uuid.NewV7(), Title: "bare"}}},
			NextCursor: &next,
		}, nil)

		s := &GRPCServer{app: app}
		resp, err := s.GetBusinessProducts(ctx, &productsv1.GetBusinessProductsRequest{
			BusinessId: businessID,
			Cursor:     &cursor,
			Limit:      10,
		})
		require.NoError(t, err)
		require.Len(t, resp.GetProducts(), 2)
		require.Equal(t, next, resp.GetNextCursor())

		got := resp.GetProducts()[0]
		require.Equal(t, p.ProductID.String(), got.GetProductId())
		require.Equal(t, "title", got.GetTitle())
		require.Equal(t, "desc", got.GetDescription())
		require.Equal(t, int64(1500), got.GetPrice())
		require.Equal(t, "http://url", got.GetImageUrl())
		require.Equal(t, 4.5, got.GetRating())
		require.Equal(t, int64(2), got.GetFeedbacksCount())
		require.Equal(t, createdAt, got.GetCreatedAt().AsTime())
		require.Equal(t, updatedAt, got.GetUpdatedAt().AsTime())

		bare := resp.GetProducts()[1]
		require.Nil(t, bare.Description, "no description - field absent")
		require.Nil(t, bare.ImageUrl, "no image - field absent")
	})

	t.Run("last page - no next cursor", func(t *testing.T) {
		app := mocks.NewMockGetBusinessItemsCase(gomock.NewController(t))
		app.EXPECT().GetBusinessItems(ctx, businessID, (*string)(nil), int64(0)).Return(&dto.Page{}, nil)

		s := &GRPCServer{app: app}
		resp, err := s.GetBusinessProducts(ctx, &productsv1.GetBusinessProductsRequest{BusinessId: businessID})
		require.NoError(t, err)
		require.Empty(t, resp.GetProducts())
		require.Nil(t, resp.NextCursor)
	})

	t.Run("invalid argument", func(t *testing.T) {
		app := mocks.NewMockGetBusinessItemsCase(gomock.NewController(t))
		app.EXPECT().GetBusinessItems(ctx, "bad", (*string)(nil), int64(0)).Return(nil, dto.ErrInvalidData)

		s := &GRPCServer{app: app}
		_, err := s.GetBusinessProducts(ctx, &productsv1.GetBusinessProductsRequest{BusinessId: "bad"})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("internal", func(t *testing.T) {
		app := mocks.NewMockGetBusinessItemsCase(gomock.NewController(t))
		app.EXPECT().GetBusinessItems(ctx, businessID, (*string)(nil), int64(0)).Return(nil, errors.New("db down"))

		s := &GRPCServer{app: app}
		_, err := s.GetBusinessProducts(ctx, &productsv1.GetBusinessProductsRequest{BusinessId: businessID})
		require.Equal(t, codes.Internal, status.Code(err))
		require.NotContains(t, err.Error(), "db down", "internal error details must not leak")
	})
}
