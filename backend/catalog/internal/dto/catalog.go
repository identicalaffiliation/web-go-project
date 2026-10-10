package dto

import (
	"errors"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

var (
	ErrInvalidJSON = errors.New("invalid JSON")
	ErrInvalidData = errors.New("invalid data")
)

type ImageFormat string

const (
	JPEG ImageFormat = "jpeg"
	PNG  ImageFormat = "png"
)

var allowedImageFormats = map[ImageFormat]string{
	JPEG: ".jpg",
	PNG:  ".png",
}

func (f ImageFormat) Ext() (string, bool) {
	ext, ok := allowedImageFormats[f]
	return ext, ok
}

type CreateProductRequest struct {
	Title       string  `json:"title" validate:"required,min=1"`
	Description *string `json:"description,omitempty"`
	Price       int64   `json:"price" validate:"required,gt=0"`
}

func (req *CreateProductRequest) ToDomain() *domain.Product {
	model := &domain.Product{
		ID:    uuid.New(),
		Title: req.Title,
		Price: req.Price,
	}

	if req.Description != nil {
		model.Description = *req.Description
	}

	return model
}

type Page struct {
	Products   []Product `json:"products"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}

type ProductResponse struct {
	Product Product `json:"product"`
}

type Product struct {
	ProductID    uuid.UUID `json:"productId"`
	Title        string    `json:"title"`
	Description  *string   `json:"description,omitempty"`
	Price        int64     `json:"price"`
	PresignedURL *string   `json:"url,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
