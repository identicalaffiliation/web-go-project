//go:build unit

package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepository_NewRepository(t *testing.T) {
	r := NewRepository(nil, nil)
	require.NotNil(t, r)
}
