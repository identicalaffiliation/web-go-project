package domain

import (
	"errors"
	"uuid"
)

var (
	ErrInvalidReservation = errors.New("invalid reservation")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrProductNotFound    = errors.New("product not found")
	// ErrOrderReleased - резерв пришел после компенсации (релиза) по этому заказу.
	ErrOrderReleased = errors.New("order stock already released")
)

// StockReservation - входные данные temporal активити резервирования и релиза стока.
// В заказе один товар в количестве Quantity.
type StockReservation struct {
	OrderID   uuid.UUID `json:"orderId"`
	ProductID uuid.UUID `json:"productId"`
	Quantity  int16     `json:"quantity"`
}

func (r *StockReservation) Validate() error {
	if r == nil || r.OrderID == uuid.Nil() || r.ProductID == uuid.Nil() || r.Quantity <= 0 {
		return ErrInvalidReservation
	}

	return nil
}
