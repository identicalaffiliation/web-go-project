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

	domainProduct, err := req.ToDomain()
	if err != nil {
		return nil, err
	}

	var created *domain.Product
	err = s.manager.Do(ctx, func(ctx context.Context) error {
		product, err := s.catalog.Insert(ctx, domainProduct)
		if err != nil {
			return err
		}

		product.Quantity = domainProduct.Quantity

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

	return s.toPage(ctx, items, limit), nil
}

// GetBusinessItems - товары, выложенные бизнесом (businessID - его user_id), страница с курсором.
func (s *Service) GetBusinessItems(ctx context.Context, businessID string, cursor *string, limit int64) (*dto.Page, error) {
	id, err := uuid.Parse(businessID)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	if limit < 1 || limit > 100 {
		limit = 50
	}

	items, err := s.catalog.GetItemsByBusinessID(ctx, id, cursor, limit+1)
	if err != nil {
		if errors.Is(err, catalog.ErrInvalidData) {
			return nil, dto.ErrInvalidData
		}

		s.logger.WithError(err).Error(
			"failed to get business items",
			zap.String("business id", id.String()),
		)
		return nil, ErrInternal
	}

	return s.toPage(ctx, items, limit), nil
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
		return s.toResponseOne(cached, s.fetchImageURL(ctx, cached)), nil
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

	return s.toResponseOne(item, s.fetchImageURL(ctx, item)), nil
}

// UploadImage - pre-signed URL для загрузки картинки товара его владельцем (бизнесом).
// Чужой или удалённый товар - ErrNotFound.
func (s *Service) UploadImage(ctx context.Context, businessID, id string, format dto.ImageFormat) (*dto.ProductResponse, error) {
	bID, err := uuid.Parse(businessID)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, dto.ErrInvalidData
	}

	ext, ok := format.Ext()
	if !ok {
		return nil, dto.ErrInvalidData
	}

	imageKey := buildImageKey(ext)

	product, err := s.catalog.UpdateImageKey(ctx, imageKey, productID, bID)
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

// UpdateProduct - изменение названия, описания или цены товара его владельцем (бизнесом).
// Чужой или удалённый товар - ErrNotFound. Кэш карточки перезаписывается сразу: из него orders берут цену.
func (s *Service) UpdateProduct(ctx context.Context, req *dto.UpdateProductRequest) (*dto.ProductResponse, error) {
	if err := req.ValidateData(); err != nil {
		return nil, err
	}

	patch, err := req.ToDomain()
	if err != nil {
		return nil, err
	}

	product, err := s.catalog.UpdateProduct(ctx, patch)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return nil, ErrNotFound
		}

		if errors.Is(err, catalog.ErrInvalidData) {
			return nil, dto.ErrInvalidData
		}

		s.logger.WithError(err).Error(
			"failed to update product",
			zap.String("product id", patch.ProductID.String()),
		)
		return nil, ErrInternal
	}

	if err := s.cache.Set(ctx, product.ID.String(), product); err != nil {
		s.logger.WithError(err).Error(
			"failed to set product to cache",
			zap.String("product id", product.ID.String()),
		)
	}

	return s.toResponseOne(product, s.fetchImageURL(ctx, product)), nil
}

// DeleteProduct - мягкое удаление товара его владельцем (бизнесом): товар пропадает из каталога,
// карточки и цены для заказов. Чужой или уже удалённый товар - ErrNotFound.
func (s *Service) DeleteProduct(ctx context.Context, businessID, productID string) error {
	bID, err := uuid.Parse(businessID)
	if err != nil {
		return dto.ErrInvalidData
	}

	pID, err := uuid.Parse(productID)
	if err != nil {
		return dto.ErrInvalidData
	}

	if err := s.catalog.DeleteProduct(ctx, pID, bID); err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return ErrNotFound
		}

		s.logger.WithError(err).Error(
			"failed to delete product",
			zap.String("product id", pID.String()),
		)
		return ErrInternal
	}

	// товар уже удалён в БД - ошибка кэша не откатывает удаление, карточка уйдёт из кэша по TTL
	if err := s.cache.Delete(ctx, pID.String()); err != nil {
		s.logger.WithError(err).Error(
			"failed to delete product from cache",
			zap.String("product id", pID.String()),
		)
	}

	return nil
}

func (s *Service) LikeItem(ctx context.Context, id string, action dto.LikeAction) error {
	productID, err := uuid.Parse(id)
	if err != nil {
		return ErrInvalid
	}

	if !action.Valid() {
		return ErrInvalid
	}

	event, err := s.toLikeEvent(productID, action)
	if err != nil {
		s.logger.WithError(err).Error("failed to mapping event")
		return err
	}

	if err := s.likes.Insert(ctx, event); err != nil {
		s.logger.WithError(err).Error("failed to create like event")
		return err
	}

	return nil
}
