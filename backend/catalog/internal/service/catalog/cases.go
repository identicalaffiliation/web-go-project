package catalog

import (
	"context"
	"errors"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/cache/redis"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"go.uber.org/zap"
)

func (s *Service) AddProductToCatalog(ctx context.Context, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
	if err := req.ValidateData(); err != nil {
		return nil, err
	}

	var created *domain.Product
	err := s.manager.Do(ctx, func(ctx context.Context) error {
		product := req.ToDomain()

		product, err := s.catalog.Insert(ctx, product)
		if err != nil {
			return err
		}

		event, err := s.toEvent(product)
		if err != nil {
			return err
		}

		if err = s.outbox.Insert(ctx, event); err != nil {
			return err
		}

		created = product
		return nil
	})
	if err != nil {
		if errors.Is(err, catalog.ErrInvalidData) {
			return nil, err
		}

		s.logger.WithError(err).Error("failed to add product to catalog")
		return nil, ErrInternal
	}

	return s.toResponseOne(created, ""), nil
}

func (s *Service) GetItemsPage(ctx context.Context, cursor *string, limit int64) (*dto.Page, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}

	items, err := s.catalog.GetItemsByCursorPagination(ctx, cursor, limit+1)
	if err != nil {
		s.logger.WithError(err).Error("failed to get items with cursor pagination")
		return nil, ErrInternal
	}

	hasMore := int64(len(items)) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		c := items[len(items)-1].ID.String()
		nextCursor = &c
	}

	return s.getPage(items, nextCursor), nil
}

func (s *Service) GetItem(ctx context.Context, id string) (*dto.ProductResponse, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	cached, err := s.cache.Get(ctx, productID.String())
	if err != nil {
		if !errors.Is(err, redis.ErrMiss) {
			s.logger.WithError(err).Error(
				"failed to get value from redis",
				zap.String("product id", productID.String()),
			)
		}
	}

	if cached != nil {
		return s.toResponseOne(cached, cached.ImageKey), nil
	}

	item, err := s.catalog.GetItemByID(ctx, productID)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return &dto.ProductResponse{}, nil
		}

		s.logger.WithError(err).Error(
			"failed to get item by id",
			zap.String("product id", productID.String()),
		)
		return nil, ErrInternal
	}

	if err := s.cache.Set(ctx, item.ID.String(), item); err != nil {
		s.logger.WithError(err).Error(
			"failed to set item to redis",
			zap.String("product id", item.ID.String()),
		)
	}

	return s.toResponseOne(item, item.ImageKey), nil
}

func (s *Service) UploadImage(ctx context.Context, id string, format dto.ImageFormat) (*dto.ProductResponse, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	ext, ok := format.Ext()
	if !ok {
		return nil, dto.ErrInvalidData
	}

	imageKey := buildImageKey(ext)

	product, err := s.catalog.UpdateImageKey(ctx, imageKey, productID)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return nil, ErrNotFound
		}

		s.logger.WithError(err).Error(
			"failed to update image key",
			zap.String("product id", productID.String()),
		)
		return nil, ErrInternal
	}

	if err := s.cache.Set(ctx, product.ID.String(), product); err != nil {
		s.logger.WithError(err).Error("failed to set product to cache")
	}

	url, err := s.minioClient.GetPresignedURL(ctx, product)
	if err != nil {
		s.logger.WithError(err).Error(
			"failed to get presigned url",
			zap.String("product id", productID.String()),
		)
		return nil, ErrInternal
	}

	return s.toResponseOne(product, url), nil
}
