package likes_outbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLikesRepository_NewRepository(t *testing.T) {
	r := NewRepository(nil, nil)
	require.NotNil(t, r)
}
