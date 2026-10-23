//go:build integration

package integration

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/adapters/catalog"
)

func TestCatalogClient_GetPrice(t *testing.T) {
	fake, conn := startFakeCatalog(t)
	client := catalog.NewClient(conn)

	t.Run("returns price", func(t *testing.T) {
		productID := uuid.NewV7()
		fake.setPrice(productID, 1500)

		price, err := client.GetPrice(t.Context(), productID)
		require.NoError(t, err)
		require.Equal(t, int64(1500), price)
	})

	t.Run("unknown product", func(t *testing.T) {
		_, err := client.GetPrice(t.Context(), uuid.NewV7())
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}
