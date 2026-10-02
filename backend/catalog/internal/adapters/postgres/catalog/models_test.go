//go:build unit

package catalog

import (
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

func Test_ProductModel_ToDomain(t *testing.T) {
	id := uuid.New()

	model := &productModel{
		ID:          id,
		Title:       "some title",
		Description: sql.Null[string]{V: "some description", Valid: true},
		Price:       100,
		ImageKey:    sql.Null[string]{Valid: false},
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	domainModel := model.toDomain()
	require.NotNil(t, domainModel)
	require.Equal(t, id, domainModel.ID)
	require.Equal(t, "", domainModel.ImageKey)
	require.Equal(t, "some description", domainModel.Description)
	require.Equal(t, false, domainModel.UpdatedAt.IsZero())
	require.Equal(t, false, domainModel.CreatedAt.IsZero())
}
