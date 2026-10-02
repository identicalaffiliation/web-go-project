package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
)

var ErrInternal = errors.New("internal error")

func (s *Service) toEvent(product *domain.Product) (*domain.Event, error) {
	payload, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("error MarshalEvent: %w", err)
	}

	return &domain.Event{
		EventID: product.ID,
		Key:     uuid.New(),
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
