package postgres

import (
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
)

type orderModel struct {
	ID          uuid.UUID     `db:"id"`
	UserID      uuid.UUID     `db:"user_id"`
	ProductID   uuid.UUID     `db:"product_id"`
	OperationID uuid.UUID     `db:"operation_id"`
	Quantity    int16         `db:"quantity"`
	TotalPrice  int64         `db:"total_price"`
	Status      domain.Status `db:"status"`
	CreatedAt   time.Time     `db:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at"`
}

func (m orderModel) toDomain() *domain.Order {
	return &domain.Order{
		ID:          m.ID,
		ProductID:   m.ProductID,
		UserID:      m.UserID,
		OperationID: m.OperationID,
		Quantity:    m.Quantity,
		TotalPrice:  m.TotalPrice,
		Status:      m.Status,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
