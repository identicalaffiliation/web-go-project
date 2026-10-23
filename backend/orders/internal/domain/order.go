package domain

import (
	"time"
	"uuid"

	ordersv1 "github.com/identicalaffiliation/web-go-project/backend/orders/gen/pb/orders/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	Created Status = "created"
	Paid    Status = "paid"
	Failed  Status = "failed"
)

var integerStatuses = map[Status]int32{
	Created: 0,
	Paid:    1,
	Failed:  2,
}

type Status string

type Order struct {
	ID          uuid.UUID `json:"id"`
	ProductID   uuid.UUID `json:"productId"`
	UserID      uuid.UUID `json:"userId"`
	OperationID uuid.UUID `json:"-"`
	Quantity    int16     `json:"quantity"`
	TotalPrice  int64     `json:"totalPrice"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (o *Order) ToResponseRPC() *ordersv1.UserOrderResponse {
	return &ordersv1.UserOrderResponse{
		OrderId:    o.ID.String(),
		ProductId:  o.ProductID.String(),
		UserId:     o.UserID.String(),
		Quantity:   int64(o.Quantity),
		TotalPrice: o.TotalPrice,
		Status:     ordersv1.OrderStatus(integerStatuses[o.Status]),
		CreatedAt:  timestamppb.New(o.CreatedAt),
		UpdatedAt:  timestamppb.New(o.UpdatedAt),
	}
}
