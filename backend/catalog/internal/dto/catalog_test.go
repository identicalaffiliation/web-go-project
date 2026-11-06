//go:build unit

package dto

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestToDomain(t *testing.T) {
	t.Run("file - nil", func(t *testing.T) {
		businessID := uuid.NewV7()
		r := &CreateProductRequest{
			Title:       "aa",
			Description: new("bb"),
			Price:       123,
			Quantity:    10,
			BusinessID:  businessID.String(),
		}

		d, err := r.ToDomain()
		require.NoError(t, err)
		require.NotNil(t, d)
		require.Equal(t, businessID, d.BusinessID)
		require.Equal(t, r.Title, d.Title)
		require.Equal(t, *r.Description, d.Description)
		require.Equal(t, r.Price, d.Price)
		require.Equal(t, r.Quantity, d.Quantity)
		require.Equal(t, "", d.ImageKey)
		require.Empty(t, d.CreatedAt)
		require.Empty(t, d.UpdatedAt)
		require.NotEmpty(t, d.ID)
	})

	t.Run("invalid business id", func(t *testing.T) {
		for _, id := range []string{"", "bad"} {
			r := &CreateProductRequest{Title: "aa", Price: 123, Quantity: 10, BusinessID: id}

			d, err := r.ToDomain()
			require.ErrorIs(t, err, ErrInvalidData)
			require.Nil(t, d)
		}
	})
}
