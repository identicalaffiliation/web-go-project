package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/catalog"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
)

func (s *Service) AddProductToCatalog(ctx context.Context, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
	if err := req.ValidateData(); err != nil {
		return nil, err
	}

	var created *domain.Product
	err := s.manager.Do(ctx, func(ctx context.Context) error {
		product := req.ToDomain()
		fmt.Println(product)

		product, err := s.catalog.Insert(ctx, product)
		if err != nil {
			return err
		}

		event, err := s.toEvent(product)
		if err != nil {
			return err
		}

		if err = s.outbox.Insert(ctx, event); err != nil {
			fmt.Println(err)
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

	url, err := s.minioClient.GetPresignedURL(ctx, created)
	if err != nil {
		s.logger.WithError(err).Error("failed to get presigned link from s3")
		return nil, ErrInternal
	}

	return s.toResponseOne(created, url), nil
}
