package dto

import (
	"fmt"
	"io"
	"net/http"
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

	if req.File != nil {
		if req.File.header.Size > maxImageSize {
			return ErrInvalidData
		}

		buf := make([]byte, 512)
		n, err := req.File.file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error Read first 512 bytes from file: %w", err)
		}

		realType := http.DetectContentType(buf[:n])
		if _, ok := allowedMimes[MimeType(realType)]; !ok {
			return ErrInvalidData
		}

		if _, err := req.File.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("error Returning Seek to start position: %w", err)
		}
	}

	return nil
}
