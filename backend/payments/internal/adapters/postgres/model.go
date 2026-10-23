package postgres

import (
	"time"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

type accountModel struct {
	UserID    uuid.UUID `db:"user_id"`
	Balance   int64     `db:"balance"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (m accountModel) toDomain() *domain.Account {
	return &domain.Account{
		UserID:    m.UserID,
		Balance:   m.Balance,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
