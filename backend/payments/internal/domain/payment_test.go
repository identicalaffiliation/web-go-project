//go:build unit

package domain

import (
	"encoding/json"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func validPayment() *Payment {
	return &Payment{OrderID: uuid.NewV7(), UserID: uuid.NewV7(), Amount: 300}
}

func TestPayment_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, validPayment().Validate())
	})

	t.Run("nil", func(t *testing.T) {
		var p *Payment
		require.ErrorIs(t, p.Validate(), ErrInvalidPayment)
	})

	cases := map[string]func(p *Payment){
		"nil order id":    func(p *Payment) { p.OrderID = uuid.Nil() },
		"nil user id":     func(p *Payment) { p.UserID = uuid.Nil() },
		"zero amount":     func(p *Payment) { p.Amount = 0 },
		"negative amount": func(p *Payment) { p.Amount = -1 },
	}

	for name, modify := range cases {
		t.Run(name, func(t *testing.T) {
			p := validPayment()
			modify(p)
			require.ErrorIs(t, p.Validate(), ErrInvalidPayment)
		})
	}
}

// json контракт с сагой orders: orders.PaymentInput
func TestPayment_JSONFieldNames(t *testing.T) {
	value := []byte(`{
		"orderId": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d",
		"userId": "0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e",
		"amount": 300
	}`)

	var p Payment
	require.NoError(t, json.Unmarshal(value, &p))
	require.Equal(t, uuid.MustParse("0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1d"), p.OrderID)
	require.Equal(t, uuid.MustParse("0192f0c1-2d3e-7a4b-8c5d-6e7f8a9b0c1e"), p.UserID)
	require.Equal(t, int64(300), p.Amount)
}
