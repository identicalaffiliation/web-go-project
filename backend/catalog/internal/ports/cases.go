package ports

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
)

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type AddToCatalogCase interface {
	AddProductToCatalog(ctx context.Context, req *dto.CreateProductRequest) (*dto.ProductResponse, error)
}
