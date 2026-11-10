package dto

import (
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate = sync.OnceValue[*validator.Validate](func() *validator.Validate {
		return validator.New()
	})
)

func (req *CreateProductRequest) ValidateJSON() error {
	if err := validate().Struct(req); err != nil {
		return ErrInvalidJSON
	}

	return nil
}

func (req *CreateProductRequest) ValidateData() error {
	if len(strings.TrimSpace(req.Title)) == 0 {
		return ErrInvalidData
	}

	if req.Description != nil {
		if len(strings.TrimSpace(*req.Description)) == 0 {
			return ErrInvalidData
		}
	}

	return nil
}

func (req *UpdateProductRequest) ValidateJSON() error {
	if err := validate().Struct(req); err != nil {
		return ErrInvalidJSON
	}

	return nil
}

func (req *UpdateProductRequest) ValidateData() error {
	if req.Title == nil && req.Description == nil && req.Price == nil {
		return ErrInvalidData
	}

	if req.Title != nil && len(strings.TrimSpace(*req.Title)) == 0 {
		return ErrInvalidData
	}

	if req.Description != nil && len(strings.TrimSpace(*req.Description)) == 0 {
		return ErrInvalidData
	}

	return nil
}

func (req *FeedbackCreateRequest) Validate() error {
	if err := validate().Struct(req); err != nil {
		return ErrInvalidJSON
	}

	if req.Description != nil {
		if len(strings.TrimSpace(*req.Description)) == 0 {
			return ErrInvalidData
		}
	}

	return nil
}
