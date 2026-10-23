package domain

import (
	"errors"
	"uuid"
)

const (
	Charge Operation = "charge"
	Refund Operation = "refund"
)

type Operation string

var (
	ErrInvalidPayment    = errors.New("invalid payment")
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrOrderRefunded - списание пришло после компенсации (возврата) по этому заказу.
	ErrOrderRefunded = errors.New("order payment already refunded")
)

// Payment - входные данные temporal активити списания и возврата.
// Amount - полная стоимость заказа.
type Payment struct {
	OrderID uuid.UUID `json:"orderId"`
	UserID  uuid.UUID `json:"userId"`
	Amount  int64     `json:"amount"`
}

func (p *Payment) Validate() error {
	if p == nil || p.OrderID == uuid.Nil() || p.UserID == uuid.Nil() || p.Amount <= 0 {
		return ErrInvalidPayment
	}

	return nil
}

// PaymentOperation - запись журнала операций по заказу.
type PaymentOperation struct {
	OrderID   uuid.UUID
	Operation Operation
	UserID    uuid.UUID
	Amount    int64
}
