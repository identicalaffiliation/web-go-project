//go:build unit

package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToDomain(t *testing.T) {
	t.Run("file - nil", func(t *testing.T) {
		r := &CreateProductRequest{
			Title:       "aa",
			Description: new("bb"),
			Price:       123,
		}

		d := r.ToDomain()
		require.NotNil(t, d)
		require.Equal(t, r.Title, d.Title)
		require.Equal(t, *r.Description, d.Description)
		require.Equal(t, r.Price, d.Price)
		require.Equal(t, "", d.ImageKey)
		require.Empty(t, d.CreatedAt)
		require.Empty(t, d.UpdatedAt)
		require.NotEmpty(t, d.ID)
	})
}
