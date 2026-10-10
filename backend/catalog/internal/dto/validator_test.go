//go:build unit

package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateProductRequest_ValidateJSON(t *testing.T) {

	t.Run("valid request", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "some title",
			Price: 100,
		}
		require.NoError(t, req.ValidateJSON())
	})

	t.Run("missing title", func(t *testing.T) {
		req := &CreateProductRequest{
			Price: 100,
		}
		err := req.ValidateJSON()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidJSON)
	})

	t.Run("empty title", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "",
			Price: 100,
		}
		err := req.ValidateJSON()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidJSON)
	})

	t.Run("zero price", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "some title",
			Price: 0,
		}
		err := req.ValidateJSON()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidJSON)
	})

	t.Run("negative price", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "some title",
			Price: -1,
		}
		err := req.ValidateJSON()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidJSON)
	})
}

func TestCreateProductRequest_ValidateData(t *testing.T) {

	t.Run("valid without file", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "some title",
			Price: 100,
		}
		require.NoError(t, req.ValidateData())
	})

	t.Run("empty title", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "",
			Price: 100,
		}
		err := req.ValidateData()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("whitespace only title", func(t *testing.T) {
		req := &CreateProductRequest{
			Title: "   ",
			Price: 100,
		}
		err := req.ValidateData()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("nil description - valid", func(t *testing.T) {
		req := &CreateProductRequest{
			Title:       "some title",
			Description: nil,
			Price:       100,
		}
		require.NoError(t, req.ValidateData())
	})

	t.Run("empty description", func(t *testing.T) {
		desc := ""
		req := &CreateProductRequest{
			Title:       "some title",
			Description: &desc,
			Price:       100,
		}
		err := req.ValidateData()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidData)
	})

	t.Run("whitespace only description", func(t *testing.T) {
		desc := "   "
		req := &CreateProductRequest{
			Title:       "some title",
			Description: &desc,
			Price:       100,
		}
		err := req.ValidateData()
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidData)
	})
}
