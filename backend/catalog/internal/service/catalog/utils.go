package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
	"go.uber.org/zap"
)

var (
	ErrInternal = errors.New("internal error")
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid data")
)

func buildImageKey(ext string) string {
	now := time.Now().UTC()
	return path.Join("image", strconv.Itoa(now.Year()), now.Month().String(), uuid.New().String()+ext)
}

// fetchImageURL - метод получения presigned ссылки на скачивание картинки товара из MinIO.
// При ошибке логирует её и возвращает пустую строку, чтобы товар отдавался без картинки
func (s *Service) fetchImageURL(ctx context.Context, product *domain.Product) string {
	url, err := s.minioClient.FetchSignedURL(ctx, product.ImageKey)
	if err != nil {
		s.logger.WithError(err).Error(
			"failed to fetch signed url",
			zap.String("product id", product.ID.String()),
		)
		return ""
	}

	return url
}

// toEvent - метод преобразования domain Product структуры в domain Event структуру для дальнейшей
// оптравки в Kafka
func (s *Service) toEvent(product *domain.Product) (*domain.Event, error) {
	payload, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("error MarshalEvent: %w", err)
	}

	// incorrect logic (key per partition => need to productID instead random id)
	// + idempotency consumer with processed events ('cause at-least-once)
	return &domain.Event{
		EventID: product.ID,
		Key:     uuid.NewV7(),
		Payload: payload,
	}, nil
}

// toLikeEvent - метод формирования события лайка товара (поставлен/убран) для отправки в Kafka.
func (s *Service) toLikeEvent(id uuid.UUID, action dto.LikeAction) (*domain.Event, error) {
	payload, err := json.Marshal(domain.LikePayload{
		ProductID: id,
		Action:    string(action),
	})
	if err != nil {
		return nil, fmt.Errorf("error MarshalEvent: %w", err)
	}

	return &domain.Event{
		EventID: uuid.NewV7(),
		Key:     id,
		Payload: payload,
	}, nil
}

func (s *Service) toResponseOne(product *domain.Product, url string) *dto.ProductResponse {
	var (
		d *string = nil
		u *string = nil
	)

	if product.Description != "" {
		d = &product.Description
	}

	if url != "" {
		u = &url
	}

	model := dto.Product{
		ProductID:    product.ID,
		Title:        product.Title,
		Description:  d,
		Price:        product.Price,
		PresignedURL: u,
		CreatedAt:    product.CreatedAt,
		UpdatedAt:    product.UpdatedAt,
	}

	return &dto.ProductResponse{
		Product: model,
	}
}

func (s *Service) getPage(products []*domain.Product, urls []string, nextCursor *string) *dto.Page {
	m := make([]dto.ProductResponse, 0, len(products))
	for i, v := range products {
		var (
			d *string
			u *string
		)

		if v.Description != "" {
			d = &v.Description
		}

		if urls[i] != "" {
			u = &urls[i]
		}

		m = append(m, dto.ProductResponse{
			Product: dto.Product{
				ProductID:    v.ID,
				Title:        v.Title,
				Description:  d,
				PresignedURL: u,
				Price:        v.Price,
				CreatedAt:    v.CreatedAt,
				UpdatedAt:    v.UpdatedAt,
			},
		})
	}

	return &dto.Page{Products: m, NextCursor: nextCursor}
}
