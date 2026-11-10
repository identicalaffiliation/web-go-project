//go:build unit

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/gen/mocks"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	catalogRepo "github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type deps struct {
	outbox  *mocks.MockOutboxRepository
	likes   *mocks.MockOutboxRepository
	catalog *mocks.MockCatalogRepository
	manager *mocks.MockTxManager
	s3      *mocks.MockS3Client
	cache   *mocks.MockCache
}

func newService(t *testing.T) (*Service, *deps) {
	ctrl := gomock.NewController(t)
	d := &deps{
		outbox:  mocks.NewMockOutboxRepository(ctrl),
		likes:   mocks.NewMockOutboxRepository(ctrl),
		catalog: mocks.NewMockCatalogRepository(ctrl),
		manager: mocks.NewMockTxManager(ctrl),
		s3:      mocks.NewMockS3Client(ctrl),
		cache:   mocks.NewMockCache(ctrl),
	}

	// транзакция в тестах - просто вызов fn
	d.manager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	).AnyTimes()

	s := NewService(d.outbox, d.catalog, d.likes, logger.NewNop(), d.manager, d.s3, d.cache)
	return s, d
}

func product() *domain.Product {
	return &domain.Product{
		ID:             uuid.NewV7(),
		Title:          "title",
		Description:    "desc",
		Price:          100,
		ImageKey:       "image/key.png",
		Rating:         4.5,
		FeedbacksCount: 2,
	}
}

func TestAddProductToCatalog(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7()
	req := func() *dto.CreateProductRequest {
		return &dto.CreateProductRequest{Title: "title", Price: 100, Quantity: 5, BusinessID: businessID.String()}
	}

	t.Run("ok - product and stock event in one tx", func(t *testing.T) {
		s, d := newService(t)
		created := product()

		d.catalog.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, p *domain.Product) (*domain.Product, error) {
				require.Equal(t, businessID, p.BusinessID, "owner from request goes to catalog")
				return created, nil
			},
		)
		d.outbox.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, e *domain.Event) error {
				var p domain.Product
				require.NoError(t, json.Unmarshal(e.Payload, &p))
				require.Equal(t, created.ID, p.ID)
				require.Equal(t, int16(5), p.Quantity, "quantity from request goes to stock event")
				return nil
			},
		)

		resp, err := s.AddProductToCatalog(ctx, req())
		require.NoError(t, err)
		require.Equal(t, created.ID, resp.Product.ProductID)
		require.Nil(t, resp.Product.PresignedURL)
	})

	t.Run("invalid business id - invalid data, nothing written", func(t *testing.T) {
		for _, id := range []string{"", "bad"} {
			s, _ := newService(t)
			r := req()
			r.BusinessID = id

			_, err := s.AddProductToCatalog(ctx, r)
			require.ErrorIs(t, err, dto.ErrInvalidData)
		}
	})

	t.Run("blank title - invalid data", func(t *testing.T) {
		s, _ := newService(t)
		r := req()
		r.Title = "   "

		_, err := s.AddProductToCatalog(ctx, r)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("check constraint from repo", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil, catalogRepo.ErrInvalidData)

		_, err := s.AddProductToCatalog(ctx, req())
		require.ErrorIs(t, err, catalogRepo.ErrInvalidData)
	})

	t.Run("outbox error - internal", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(product(), nil)
		d.outbox.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.New("db down"))

		_, err := s.AddProductToCatalog(ctx, req())
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestGetItem(t *testing.T) {
	ctx := context.Background()

	t.Run("cache hit - db not called", func(t *testing.T) {
		s, d := newService(t)
		p := product()

		d.cache.EXPECT().Get(ctx, p.ID.String()).Return(p, nil)
		d.s3.EXPECT().FetchSignedURL(ctx, p.ImageKey).Return("http://url", nil)

		resp, err := s.GetItem(ctx, p.ID.String())
		require.NoError(t, err)
		require.Equal(t, p.ID, resp.Product.ProductID)
		require.Equal(t, "http://url", *resp.Product.PresignedURL)
		require.Equal(t, 4.5, resp.Product.Rating)
		require.Equal(t, int64(2), resp.Product.FeedbacksCount)
	})

	t.Run("cache miss - read db and set cache", func(t *testing.T) {
		s, d := newService(t)
		p := product()

		d.cache.EXPECT().Get(ctx, p.ID.String()).Return(nil, redis.ErrMiss)
		d.catalog.EXPECT().GetItemByID(ctx, p.ID).Return(p, nil)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(nil)
		d.s3.EXPECT().FetchSignedURL(ctx, p.ImageKey).Return("", nil)

		resp, err := s.GetItem(ctx, p.ID.String())
		require.NoError(t, err)
		require.Equal(t, p.ID, resp.Product.ProductID)
		require.Nil(t, resp.Product.PresignedURL)
	})

	t.Run("cache error is not fatal", func(t *testing.T) {
		s, d := newService(t)
		p := product()

		d.cache.EXPECT().Get(ctx, p.ID.String()).Return(nil, errors.New("redis down"))
		d.catalog.EXPECT().GetItemByID(ctx, p.ID).Return(p, nil)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(errors.New("redis down"))
		d.s3.EXPECT().FetchSignedURL(ctx, p.ImageKey).Return("", errors.New("minio down"))

		resp, err := s.GetItem(ctx, p.ID.String())
		require.NoError(t, err)
		require.Equal(t, p.ID, resp.Product.ProductID)
	})

	t.Run("not found - empty response", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()

		d.cache.EXPECT().Get(ctx, id.String()).Return(nil, redis.ErrMiss)
		d.catalog.EXPECT().GetItemByID(ctx, id).Return(nil, catalogRepo.ErrNotFound)

		resp, err := s.GetItem(ctx, id.String())
		require.NoError(t, err)
		require.Equal(t, uuid.Nil(), resp.Product.ProductID)
	})

	t.Run("invalid id", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.GetItem(ctx, "bad")
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("db error - internal", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()

		d.cache.EXPECT().Get(ctx, id.String()).Return(nil, redis.ErrMiss)
		d.catalog.EXPECT().GetItemByID(ctx, id).Return(nil, errors.New("db down"))

		_, err := s.GetItem(ctx, id.String())
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestGetItemsPage(t *testing.T) {
	ctx := context.Background()

	items := func(n int) []*domain.Product {
		out := make([]*domain.Product, n)
		for i := range out {
			out[i] = product()
		}
		return out
	}

	t.Run("has more - next cursor", func(t *testing.T) {
		s, d := newService(t)
		list := items(3)

		d.catalog.EXPECT().GetItemsByCursorPagination(ctx, (*string)(nil), int64(3)).Return(list, nil)
		d.s3.EXPECT().FetchSignedURL(ctx, gomock.Any()).Return("http://url", nil).Times(2)

		page, err := s.GetItemsPage(ctx, nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Products, 2)
		require.Equal(t, list[1].ID.String(), *page.NextCursor)
		require.Equal(t, 4.5, page.Products[0].Product.Rating)
		require.Equal(t, int64(2), page.Products[0].Product.FeedbacksCount)
	})

	t.Run("last page - no cursor", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByCursorPagination(ctx, (*string)(nil), int64(3)).Return(items(1), nil)
		d.s3.EXPECT().FetchSignedURL(ctx, gomock.Any()).Return("", nil)

		page, err := s.GetItemsPage(ctx, nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Products, 1)
		require.Nil(t, page.NextCursor)
	})

	t.Run("limit out of range - default 50", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByCursorPagination(ctx, (*string)(nil), int64(51)).Return(nil, nil)

		_, err := s.GetItemsPage(ctx, nil, 1000)
		require.NoError(t, err)
	})

	t.Run("repo error - internal", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByCursorPagination(ctx, (*string)(nil), int64(51)).Return(nil, errors.New("db down"))

		_, err := s.GetItemsPage(ctx, nil, 50)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestGetBusinessItems(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7()

	items := func(n int) []*domain.Product {
		out := make([]*domain.Product, n)
		for i := range out {
			out[i] = product()
			out[i].BusinessID = businessID
		}
		return out
	}

	t.Run("has more - next cursor", func(t *testing.T) {
		s, d := newService(t)
		list := items(3)
		cursor := uuid.NewV7().String()

		d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, &cursor, int64(3)).Return(list, nil)
		d.s3.EXPECT().FetchSignedURL(ctx, gomock.Any()).Return("http://url", nil).Times(2)

		page, err := s.GetBusinessItems(ctx, businessID.String(), &cursor, 2)
		require.NoError(t, err)
		require.Len(t, page.Products, 2)
		require.Equal(t, list[0].ID, page.Products[0].Product.ProductID)
		require.Equal(t, "http://url", *page.Products[0].Product.PresignedURL)
		require.Equal(t, list[1].ID.String(), *page.NextCursor)
	})

	t.Run("last page - no cursor", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, (*string)(nil), int64(3)).Return(items(1), nil)
		d.s3.EXPECT().FetchSignedURL(ctx, gomock.Any()).Return("", nil)

		page, err := s.GetBusinessItems(ctx, businessID.String(), nil, 2)
		require.NoError(t, err)
		require.Len(t, page.Products, 1)
		require.Nil(t, page.Products[0].Product.PresignedURL)
		require.Nil(t, page.NextCursor)
	})

	t.Run("no products - empty page", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, (*string)(nil), int64(51)).Return(nil, nil)

		page, err := s.GetBusinessItems(ctx, businessID.String(), nil, 50)
		require.NoError(t, err)
		require.Empty(t, page.Products)
		require.Nil(t, page.NextCursor)
	})

	t.Run("limit out of range - default 50", func(t *testing.T) {
		for _, limit := range []int64{0, 1000} {
			s, d := newService(t)
			d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, (*string)(nil), int64(51)).Return(nil, nil)

			_, err := s.GetBusinessItems(ctx, businessID.String(), nil, limit)
			require.NoError(t, err)
		}
	})

	t.Run("invalid business id", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.GetBusinessItems(ctx, "bad", nil, 50)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("invalid cursor - invalid data", func(t *testing.T) {
		s, d := newService(t)
		cursor := "bad"
		d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, &cursor, int64(51)).Return(nil, catalogRepo.ErrInvalidData)

		_, err := s.GetBusinessItems(ctx, businessID.String(), &cursor, 50)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("repo error - internal", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().GetItemsByBusinessID(ctx, businessID, (*string)(nil), int64(51)).Return(nil, errors.New("db down"))

		_, err := s.GetBusinessItems(ctx, businessID.String(), nil, 50)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestUploadImage(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7()

	t.Run("ok - key updated by owner, cache refreshed", func(t *testing.T) {
		s, d := newService(t)
		p := product()

		d.catalog.EXPECT().UpdateImageKey(ctx, gomock.Any(), p.ID, businessID).DoAndReturn(
			func(_ context.Context, key string, _, _ uuid.UUID) (*domain.Product, error) {
				require.Regexp(t, `^image/\d{4}/\w+/[0-9a-f-]+\.png$`, key)
				return p, nil
			},
		)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(nil)
		d.s3.EXPECT().GetPresignedURL(ctx, p).Return("http://upload", nil)

		resp, err := s.UploadImage(ctx, businessID.String(), p.ID.String(), dto.PNG)
		require.NoError(t, err)
		require.Equal(t, "http://upload", *resp.Product.PresignedURL)
	})

	t.Run("invalid id", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.UploadImage(ctx, businessID.String(), "bad", dto.PNG)
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("invalid business id", func(t *testing.T) {
		s, _ := newService(t)
		for _, id := range []string{"", "bad"} {
			_, err := s.UploadImage(ctx, id, uuid.NewV7().String(), dto.PNG)
			require.ErrorIs(t, err, dto.ErrInvalidData)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		s, _ := newService(t)
		_, err := s.UploadImage(ctx, businessID.String(), uuid.NewV7().String(), "gif")
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("not found or not owner", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()
		d.catalog.EXPECT().UpdateImageKey(ctx, gomock.Any(), id, businessID).Return(nil, catalogRepo.ErrNotFound)

		_, err := s.UploadImage(ctx, businessID.String(), id.String(), dto.JPEG)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("presign error - internal", func(t *testing.T) {
		s, d := newService(t)
		p := product()
		d.catalog.EXPECT().UpdateImageKey(ctx, gomock.Any(), p.ID, businessID).Return(p, nil)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(nil)
		d.s3.EXPECT().GetPresignedURL(ctx, p).Return("", errors.New("minio down"))

		_, err := s.UploadImage(ctx, businessID.String(), p.ID.String(), dto.JPEG)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestLikeItem(t *testing.T) {
	ctx := context.Background()
	productID := uuid.NewV7()

	t.Run("ok - event keyed by product", func(t *testing.T) {
		for _, action := range []dto.LikeAction{dto.LikeAdd, dto.LikeRemove} {
			s, d := newService(t)
			d.likes.EXPECT().Insert(ctx, gomock.Any()).DoAndReturn(
				func(_ context.Context, e *domain.Event) error {
					// ключ = товар: все события одного товара в одной партиции, порядок сохраняется
					require.Equal(t, productID, e.Key)

					var p domain.LikePayload
					require.NoError(t, json.Unmarshal(e.Payload, &p))
					require.Equal(t, productID, p.ProductID)
					require.Equal(t, string(action), p.Action)
					return nil
				},
			)

			require.NoError(t, s.LikeItem(ctx, productID.String(), action))
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		s, _ := newService(t)
		require.ErrorIs(t, s.LikeItem(ctx, "bad", dto.LikeAdd), ErrInvalid)
	})

	t.Run("invalid action", func(t *testing.T) {
		s, _ := newService(t)
		require.ErrorIs(t, s.LikeItem(ctx, productID.String(), "toggle"), ErrInvalid)
	})

	t.Run("outbox error", func(t *testing.T) {
		s, d := newService(t)
		d.likes.EXPECT().Insert(ctx, gomock.Any()).Return(errors.New("db down"))
		require.Error(t, s.LikeItem(ctx, productID.String(), dto.LikeAdd))
	})
}

func TestUpdateProduct(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7()
	req := func(id uuid.UUID) *dto.UpdateProductRequest {
		return &dto.UpdateProductRequest{
			Price:      new(int64(250)),
			ProductID:  id.String(),
			BusinessID: businessID.String(),
		}
	}

	t.Run("ok - owner patch, cache refreshed", func(t *testing.T) {
		s, d := newService(t)
		p := product()
		p.Price = 250

		d.catalog.EXPECT().UpdateProduct(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, patch *domain.ProductPatch) (*domain.Product, error) {
				require.Equal(t, p.ID, patch.ProductID)
				require.Equal(t, businessID, patch.BusinessID, "owner from request goes to catalog")
				require.Equal(t, int64(250), *patch.Price)
				require.Nil(t, patch.Title)
				require.Nil(t, patch.Description)
				return p, nil
			},
		)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(nil)
		d.s3.EXPECT().FetchSignedURL(ctx, p.ImageKey).Return("http://url", nil)

		resp, err := s.UpdateProduct(ctx, req(p.ID))
		require.NoError(t, err)
		require.Equal(t, p.ID, resp.Product.ProductID)
		require.Equal(t, int64(250), resp.Product.Price)
		require.Equal(t, "http://url", *resp.Product.PresignedURL)
	})

	t.Run("cache error is not fatal", func(t *testing.T) {
		s, d := newService(t)
		p := product()

		d.catalog.EXPECT().UpdateProduct(ctx, gomock.Any()).Return(p, nil)
		d.cache.EXPECT().Set(ctx, p.ID.String(), p).Return(errors.New("redis down"))
		d.s3.EXPECT().FetchSignedURL(ctx, p.ImageKey).Return("", nil)

		resp, err := s.UpdateProduct(ctx, req(p.ID))
		require.NoError(t, err)
		require.Equal(t, p.ID, resp.Product.ProductID)
	})

	t.Run("invalid - repo not called", func(t *testing.T) {
		cases := map[string]*dto.UpdateProductRequest{
			"nothing to change": {ProductID: uuid.NewV7().String(), BusinessID: businessID.String()},
			"bad product id":    {Price: new(int64(1)), ProductID: "bad", BusinessID: businessID.String()},
			"no business id":    {Price: new(int64(1)), ProductID: uuid.NewV7().String()},
		}

		for name, r := range cases {
			t.Run(name, func(t *testing.T) {
				s, _ := newService(t)
				_, err := s.UpdateProduct(ctx, r)
				require.ErrorIs(t, err, dto.ErrInvalidData)
			})
		}
	})

	t.Run("not found or not owner", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().UpdateProduct(ctx, gomock.Any()).Return(nil, catalogRepo.ErrNotFound)

		_, err := s.UpdateProduct(ctx, req(uuid.NewV7()))
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("check constraint - invalid", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().UpdateProduct(ctx, gomock.Any()).Return(nil, catalogRepo.ErrInvalidData)

		_, err := s.UpdateProduct(ctx, req(uuid.NewV7()))
		require.ErrorIs(t, err, dto.ErrInvalidData)
	})

	t.Run("db error - internal", func(t *testing.T) {
		s, d := newService(t)
		d.catalog.EXPECT().UpdateProduct(ctx, gomock.Any()).Return(nil, errors.New("db down"))

		_, err := s.UpdateProduct(ctx, req(uuid.NewV7()))
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestDeleteProduct(t *testing.T) {
	ctx := context.Background()
	businessID := uuid.NewV7()

	t.Run("ok - deleted and evicted from cache", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()

		d.catalog.EXPECT().DeleteProduct(ctx, id, businessID).Return(nil)
		d.cache.EXPECT().Delete(ctx, id.String()).Return(nil)

		require.NoError(t, s.DeleteProduct(ctx, businessID.String(), id.String()))
	})

	t.Run("cache error is not fatal", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()

		d.catalog.EXPECT().DeleteProduct(ctx, id, businessID).Return(nil)
		d.cache.EXPECT().Delete(ctx, id.String()).Return(errors.New("redis down"))

		require.NoError(t, s.DeleteProduct(ctx, businessID.String(), id.String()))
	})

	t.Run("invalid ids", func(t *testing.T) {
		s, _ := newService(t)
		require.ErrorIs(t, s.DeleteProduct(ctx, "bad", uuid.NewV7().String()), dto.ErrInvalidData)
		require.ErrorIs(t, s.DeleteProduct(ctx, "", uuid.NewV7().String()), dto.ErrInvalidData)
		require.ErrorIs(t, s.DeleteProduct(ctx, businessID.String(), "bad"), dto.ErrInvalidData)
	})

	t.Run("not found or not owner - cache untouched", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()
		d.catalog.EXPECT().DeleteProduct(ctx, id, businessID).Return(catalogRepo.ErrNotFound)

		require.ErrorIs(t, s.DeleteProduct(ctx, businessID.String(), id.String()), ErrNotFound)
	})

	t.Run("db error - internal", func(t *testing.T) {
		s, d := newService(t)
		id := uuid.NewV7()
		d.catalog.EXPECT().DeleteProduct(ctx, id, businessID).Return(errors.New("db down"))

		require.ErrorIs(t, s.DeleteProduct(ctx, businessID.String(), id.String()), ErrInternal)
	})
}
