package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
)

var (
	ErrInternal  = errors.New("internal error")
	ErrNotFound  = errors.New("not found")
)

func buildImageKey(ext string) string {
	now := time.Now().UTC()
	return path.Join("image", strconv.Itoa(now.Year()), now.Month().String(), uuid.New().String()+ext)
}

func (s *Service) toEvent(product *domain.Product) (*domain.Event, error) {
	payload, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("error MarshalEvent: %w", err)
	}

	return &domain.Event{
		EventID: product.ID,
		Key:     uuid.NewV7(),
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

func (s *Service) getPage(products []*domain.Product, nextCursor *string) *dto.Page {
	m := make([]dto.Product, 0, len(products))
	for _, v := range products {
		var (
			d *string
		)

		if v.Description != "" {
			d = &v.Description
		}

		m = append(m, dto.Product{
			ProductID:   v.ID,
			Title:       v.Title,
			Description: d,
			Price:       v.Price,
			CreatedAt:   v.CreatedAt,
			UpdatedAt:   v.UpdatedAt,
		})
	}

	return &dto.Page{Products: m, NextCursor: nextCursor}
}
