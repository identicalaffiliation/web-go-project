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

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type UploadImageCase interface {
	UploadImage(ctx context.Context, id string, format dto.ImageFormat) (*dto.ProductResponse, error)
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type LikeItem interface {
	LikeItem(ctx context.Context, id string, action dto.LikeAction) error
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type CreateFeedbackCase interface {
	CreateFeedback(ctx context.Context, req *dto.FeedbackCreateRequest) (*dto.FeedbackResponse, error)
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type GetProductFeedbacksCase interface {
	GetProductFeedbacks(ctx context.Context, productID string, cursor *string, limit int64) (*dto.FeedbacksPage, error)
}

//go:generate mockgen -source=$GOFILE -destination=../../gen/mocks/mock_$GOFILE -package=mocks
type FeedbackService interface {
	GetMyCreatedFeedbacks(ctx context.Context, userID string) ([]*dto.FeedbackResponse, error)
	DeleteFeedback(ctx context.Context, id string) error
}
