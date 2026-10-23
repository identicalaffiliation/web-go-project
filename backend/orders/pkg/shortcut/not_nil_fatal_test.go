//go:build unit

package shortcut

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNotNilFatal(t *testing.T) {
	require.NotPanics(t, func() { ErrNotNilFatal(nil) })
}
