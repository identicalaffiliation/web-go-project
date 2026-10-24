package postgres

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/backend/payments/internal/domain"
)

type Repository struct {
	pool     *pgxpool.Pool
	poolRO   *pgxpool.Pool
	txGetter *trm.CtxGetter
}

func NewRepository(pool, poolRO *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, poolRO: poolRO, txGetter: trm.DefaultCtxGetter}
}

// GetAccount - счет пользователя. При SELECT вне транзакции читаем с реплики,
// чтобы не нагружать master. Внутри транзакции txGetter вернет транзакцию master.
func (r *Repository) GetAccount(ctx context.Context, userID uuid.UUID) (*domain.Account, error) {
	const query = `SELECT user_id, balance, created_at, updated_at FROM accounts WHERE user_id = $1`

	rows, err := r.txGetter.DefaultTrOrDB(ctx, r.poolRO).Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select account: %w", err)
	}

	model, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[accountModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}

		return nil, fmt.Errorf("collect account: %w", err)
	}

	return model.toDomain(), nil
}

// CreateAccount - открывает счет со стартовым балансом. Повторное открытие не меняет баланс.
func (r *Repository) CreateAccount(ctx context.Context, userID uuid.UUID, balance int64) (bool, error) {
	const query = `
		INSERT INTO accounts (user_id, balance)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING
	`

	tag, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, userID, balance)
	if err != nil {
		return false, fmt.Errorf("create account: %w", validateConstraints(err))
	}

	return tag.RowsAffected() == 1, nil
}

// SaveProcessedEvent - inbox консьюмера. Запись идет в одной транзакции с открытием счета,
// поэтому повторно доставленное событие не применится дважды.
func (r *Repository) SaveProcessedEvent(ctx context.Context, eventID uuid.UUID) (bool, error) {
	const query = `
		INSERT INTO processed_events (event_id)
		VALUES ($1)
		ON CONFLICT (event_id) DO NOTHING
	`

	tag, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, eventID)
	if err != nil {
		return false, fmt.Errorf("save processed event: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}

// Debit - списывает amount одним UPDATE: строка счета блокируется до конца транзакции,
// а CHECK(balance >= 0) не дает уйти в минус при конкурентных списаниях.
func (r *Repository) Debit(ctx context.Context, userID uuid.UUID, amount int64) (int64, error) {
	const query = `
		UPDATE accounts
		SET balance = balance - $2, updated_at = NOW()
		WHERE user_id = $1
		RETURNING balance
	`

	return r.changeBalance(ctx, query, userID, amount)
}

func (r *Repository) Credit(ctx context.Context, userID uuid.UUID, amount int64) (int64, error) {
	const query = `
		UPDATE accounts
		SET balance = balance + $2, updated_at = NOW()
		WHERE user_id = $1
		RETURNING balance
	`

	return r.changeBalance(ctx, query, userID, amount)
}

func (r *Repository) changeBalance(ctx context.Context, query string, userID uuid.UUID, amount int64) (int64, error) {
	var balance int64
	err := r.txGetter.DefaultTrOrDB(ctx, r.pool).QueryRow(ctx, query, userID, amount).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrAccountNotFound
		}

		if errors.Is(validateConstraints(err), ErrInvalidData) {
			return 0, domain.ErrInsufficientFunds
		}

		return 0, fmt.Errorf("change balance: %w", err)
	}

	return balance, nil
}

// LockOrder - транзакционный advisory lock на заказ. Сериализует конкурентные
// списание/возврат одного заказа.
func (r *Repository) LockOrder(ctx context.Context, orderID uuid.UUID) error {
	const query = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`

	if _, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, orderID.String()); err != nil {
		return fmt.Errorf("lock order: %w", err)
	}

	return nil
}

// GetOperation - операция по заказу из журнала. Читает master: вызывается внутри транзакции
// и должна видеть свежие записи, лаг реплики тут недопустим.
func (r *Repository) GetOperation(
	ctx context.Context,
	orderID uuid.UUID,
	op domain.Operation,
) (*domain.PaymentOperation, bool, error) {
	const query = `
		SELECT order_id, operation, user_id, amount
		FROM payment_operations
		WHERE order_id = $1 AND operation = $2
	`

	var res domain.PaymentOperation
	err := r.txGetter.DefaultTrOrDB(ctx, r.pool).QueryRow(ctx, query, orderID, op).
		Scan(&res.OrderID, &res.Operation, &res.UserID, &res.Amount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("get payment operation: %w", err)
	}

	return &res, true, nil
}

func (r *Repository) SaveOperation(ctx context.Context, op *domain.PaymentOperation) error {
	const query = `
		INSERT INTO payment_operations (order_id, operation, user_id, amount)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, op.OrderID, op.Operation, op.UserID, op.Amount)
	if err != nil {
		return fmt.Errorf("save payment operation: %w", validateConstraints(err))
	}

	return nil
}
