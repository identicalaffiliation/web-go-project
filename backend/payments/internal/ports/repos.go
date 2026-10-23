package ports

import (
	"context"
	"uuid"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

type Repository interface {
	// GetAccount - чтение баланса вне транзакции идет с реплики.
	GetAccount(ctx context.Context, userID uuid.UUID) (*domain.Account, error)
	// CreateAccount - возвращает false, если счет пользователя уже есть.
	CreateAccount(ctx context.Context, userID uuid.UUID, balance int64) (bool, error)
	// SaveProcessedEvent - возвращает false, если событие уже обработано.
	SaveProcessedEvent(ctx context.Context, eventID uuid.UUID) (bool, error)

	Debit(ctx context.Context, userID uuid.UUID, amount int64) (int64, error)
	Credit(ctx context.Context, userID uuid.UUID, amount int64) (int64, error)

	LockOrder(ctx context.Context, orderID uuid.UUID) error
	GetOperation(ctx context.Context, orderID uuid.UUID, op domain.Operation) (*domain.PaymentOperation, bool, error)
	SaveOperation(ctx context.Context, op *domain.PaymentOperation) error
}
