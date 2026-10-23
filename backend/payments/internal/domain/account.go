package domain

import (
	"errors"
	"time"
	"uuid"

	balancev1 "github.com/identicalaffiliation/web-go-project/backend/payments/gen/pb/balance/v1"
)

var ErrAccountNotFound = errors.New("account not found")

type Account struct {
	UserID    uuid.UUID
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (a *Account) ToResponseRPC() *balancev1.GetBalanceResponse {
	return &balancev1.GetBalanceResponse{
		UserId: a.UserID.String(),
		Amount: a.Balance,
	}
}
