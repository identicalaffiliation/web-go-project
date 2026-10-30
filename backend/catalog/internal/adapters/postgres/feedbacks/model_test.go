//go:build unit

package feedbacks

import (
	"database/sql"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func Test_ToModel(t *testing.T) {
	m := &feedbackModel{
		ID:        uuid.New(),
		UserID:    uuid.NewV7(),
		ProductID: uuid.NewV7(),
		Score:     3,
	}

	require.NotNil(t, m.toDomain())
}

func Test_ToDomain_Description(t *testing.T) {
	t.Run("null - nil pointer", func(t *testing.T) {
		m := &feedbackModel{ID: uuid.NewV7(), Description: sql.Null[string]{}}
		require.Nil(t, m.toDomain().Description)
	})

	t.Run("value", func(t *testing.T) {
		m := &feedbackModel{ID: uuid.NewV7(), Description: sql.Null[string]{V: "text", Valid: true}}
		require.Equal(t, "text", *m.toDomain().Description)
	})
}
