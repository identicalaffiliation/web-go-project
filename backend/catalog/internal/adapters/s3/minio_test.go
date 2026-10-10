//go:build unit

package s3

import (
	"testing"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/config"
	"github.com/stretchr/testify/require"
)

func Test_NewMinioClient(t *testing.T) {
	cfg := new(config.MinioConfig)
	client, err := NewClient(cfg)
	require.Nil(t, client)
	require.Error(t, err)
}
