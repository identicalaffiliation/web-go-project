package dto

import (
	"errors"
	"mime/multipart"
	"path"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

var (
	allowedMimes = map[MimeType]struct{}{
		jpeg: {},
		png:  {},
	}
)

const (
	jpeg MimeType = "image/jpeg"
	png  MimeType = "image/png"

	maxImageSize = 10 << 20 // 10Mb

	s3prefix = "image"
)

type MimeType string

var (
	ErrInvalidJSON = errors.New("invalid JSON")
	ErrInvalidData = errors.New("invalid data")
)

type CreateProductRequest struct {
	Title       string  `json:"title" validate:"required,min=1"`
	Description *string `json:"description,omitempty"`
	Price       int64   `json:"price" validate:"required,gt=0"`
	File        *File
}

func (req *CreateProductRequest) ToDomain() *domain.Product {
	key := req.buildImageKeyFromImageName()
	if key == "" {
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

	model := &domain.Product{
		ID:       uuid.New(),
		Title:    req.Title,
		Price:    req.Price,
		ImageKey: key,
	}

	if req.Description != nil {
		model.Description = *req.Description
	}

	return model
}

func (req *CreateProductRequest) buildImageKeyFromImageName() string {
	if req.File == nil {
		return ""
	}

	format := "." + strings.TrimPrefix(
		req.File.header.Header.Get("Content-Type"),
		"image/",
	)

	now := time.Now().UTC()
	year := now.Year()
	month := now.Month()

	return path.Join(
		s3prefix,
		strconv.Itoa(year),
		month.String(),
		uuid.New().String()+format,
	)
}

type File struct {
	file   multipart.File
	header *multipart.FileHeader
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

func NewFile(file multipart.File, header *multipart.FileHeader) *File {
	return &File{
		file:   file,
		header: header,
	}
}
