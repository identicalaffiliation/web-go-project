package dto

import (
	"errors"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

var ErrInvalidData = errors.New("invalid data")

type CreateRequest struct {
	ProductID   uuid.UUID `json:"productId" validate:"required" format:"uuid" example:"01a16a84-ff25-7770-9542-0ca3281bb16c"`
	OperationID uuid.UUID `json:"operationId" validate:"required" format:"uuid" example:"01a16a84-ff25-7774-b48f-bc656eaf370f"`
	UserID      uuid.UUID `json:"-" validate:"required"`
	Quantity    int16     `json:"quantity" validate:"required,gt=0,lt=100" minimum:"1" maximum:"99" example:"1"`
}

func (req *CreateRequest) ToDomain() *domain.Order {
	return &domain.Order{
		ID:          uuid.NewV7(),
		ProductID:   req.ProductID,
		UserID:      req.UserID,
		OperationID: req.OperationID,
		Quantity:    req.Quantity,
	}
}

func (req *CreateRequest) Validate() error {
	if err := domain.Validate().Struct(req); err != nil {
		return ErrInvalidData
	}

	if req.ProductID == uuid.Nil() || req.OperationID == uuid.Nil() || req.UserID == uuid.Nil() {
		return ErrInvalidData
	}

	return nil
}

type OrderResponse struct {
	Order domain.Order `json:"order"`
}

func NewOrderResponse(order *domain.Order) *OrderResponse {
	return &OrderResponse{Order: *order}
}
