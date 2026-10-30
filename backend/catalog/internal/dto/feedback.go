package dto

import (
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
)

type FeedbackCreateRequest struct {
	ProductID   string  `json:"productId" validate:"required,uuid" example:"01a10112-c339-715d-8feb-4751a1ced5ef"`
	UserID      string  `json:"-" validate:"required"`
	Score       int8    `json:"score" validate:"required,min=1,max=5" example:"5"`
	Description *string `json:"description,omitempty" example:"great product"`
}

func (req *FeedbackCreateRequest) ToDomain() (*domain.Feedback, error) {
	pID, err := uuid.Parse(req.ProductID)
	if err != nil {
		return nil, ErrInvalidData
	}

	uID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, ErrInvalidData
	}

	return &domain.Feedback{
		ID:          uuid.NewV7(),
		ProductID:   pID,
		UserID:      uID,
		Score:       req.Score,
		Description: req.Description,
	}, nil
}

// FeedbacksPage - страница отзывов товара.
// NextCursor - Опциональное поле следующей пейджи.
type FeedbacksPage struct {
	Feedbacks  []*FeedbackResponse `json:"feedbacks"`
	NextCursor *string             `json:"nextCursor,omitempty"`
}

type FeedbackResponse struct {
	ID          uuid.UUID `json:"id" swaggertype:"string" format:"uuid"`
	UserID      uuid.UUID `json:"userId" swaggertype:"string" format:"uuid"`
	ProductID   uuid.UUID `json:"productId" swaggertype:"string" format:"uuid"`
	Score       int8      `json:"score" example:"5"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
