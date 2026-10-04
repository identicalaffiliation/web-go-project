package ports

import (
	"context"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/dto"
)

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type AddToCatalogCase interface {
	AddProductToCatalog(ctx context.Context, req *dto.CreateProductRequest) (*dto.ProductResponse, error)
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type GetItemCase interface {
	GetItem(ctx context.Context, id string) (*dto.ProductResponse, error)
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type GetPageCase interface {
	GetItemsPage(ctx context.Context, cursor *string, limit int64) (*dto.Page, error)
}
