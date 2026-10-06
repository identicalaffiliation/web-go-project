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
