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

// CreateProductRequest - Data transfer object структура JSON запроса для создания товара в каталоге.
// Поля:
// Title - название товара, минимальная длина - 1, поле обязательно.
// Description - описание товара, поле опционально.
// Price - цена товара, минимальное значение - 1, не может быть отрицательно, поле обязательно.
// Quantity - количество товара, диапозон количества [1, 100), поле обязательно.
type CreateProductRequest struct {
	Title       string  `json:"title" validate:"required,min=1"`
	Description *string `json:"description,omitempty"`
	Price       int64   `json:"price" validate:"required,gt=0"`
	Quantity    int16   `json:"quantity" validate:"required,gte=1,lt=100"`
}

// ToDomain - метод преобразования DTO в domain структуру.
func (req *CreateProductRequest) ToDomain() *domain.Product {
	model := &domain.Product{
		ID:       uuid.New(),
		Title:    req.Title,
		Price:    req.Price,
		Quantity: req.Quantity,
	}

	if req.Description != nil {
		model.Description = *req.Description
	}

	return model
}

// LikeProductRequest - DTO запроса лайка товара.
// ID - идентификатор товара (UUID), поле обязательно.
type LikeProductRequest struct {
	ID string `json:"id" example:"0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"`
}

// Page - структура страницы продуктов.
// NextCursor - Опциональное поле следующей пейджи.
type Page struct {
	Products   []ProductResponse `json:"products"`
	NextCursor *string           `json:"nextCursor,omitempty"`
}

// ProductResponse - основная dto структура продукта
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
