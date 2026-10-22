package domain

import (
	"time"
	"uuid"
)

type Stock struct {
	ProductID uuid.UUID
	Quantity  int16
	CreatedAt time.Time
	UpdatedAt time.Time
}

func FromDomainProductToStock(product *Product) *Stock {
	return &Stock{
		ProductID: product.ID,
		Quantity:  product.Quantity,
	}
}
