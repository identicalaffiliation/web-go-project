package stock

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/backend/inventory/internal/domain"
)

type Repository struct {
	pool     *pgxpool.Pool
	txGetter *trm.CtxGetter
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, txGetter: trm.DefaultCtxGetter}
}

func (r *Repository) CreateStock(ctx context.Context, stock *domain.Stock) error {
	const query = `INSERT INTO stock (product_id, quantity) VALUES ($1, $2)`

	pool := r.txGetter.DefaultTrOrDB(ctx, r.pool)
	if _, err := pool.Exec(ctx, query, stock.ProductID, stock.Quantity); err != nil {
		if errors.Is(ErrInvalidData, validateConstraints(err)) ||
			errors.Is(ErrAlreadyExists, validateConstraints(err)) {
			return validateConstraints(err)
		}

		return fmt.Errorf("create stock: %w", err)
	}

	return nil
}

func (r *Repository) ReserveStock(ctx context.Context, quantity int16, productID uuid.UUID) (int16, error) {
	const query = `
        UPDATE stock 
        SET quantity = quantity - $2, updated = NOW()
        WHERE product_id = $1 AND quantity >= $2
        RETURNING quantity
    `

	var newQty int16
	err := r.txGetter.DefaultTrOrDB(ctx, r.pool).QueryRow(ctx, query, productID, quantity).Scan(&newQty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrInsufficientStock
		}

		if errors.Is(validateConstraints(err), ErrInvalidData) {
			return 0, domain.ErrInsufficientStock
		}

		return 0, fmt.Errorf("reserve stock: %w", err)
	}

	return newQty, nil
}

func (r *Repository) ReleaseStock(ctx context.Context, quantity int16, productID uuid.UUID) (int16, error) {
	const query = `
        UPDATE stock 
        SET quantity = quantity + $2, updated = NOW()
        WHERE product_id = $1
        RETURNING quantity
    `

	var newQty int16
	err := r.txGetter.DefaultTrOrDB(ctx, r.pool).QueryRow(ctx, query, productID, quantity).Scan(&newQty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrProductNotFound
		}

		if errors.Is(validateConstraints(err), ErrInvalidData) {
			return 0, domain.ErrInvalidReservation
		}

		return 0, fmt.Errorf("release stock: %w", err)
	}

	return newQty, nil
}

// LockOrder - транзакционный advisory lock на заказ. Сериализует конкурентные
// резерв/релиз одного заказа
func (r *Repository) LockOrder(ctx context.Context, orderID uuid.UUID) error {
	const query = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`

	if _, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, orderID.String()); err != nil {
		return fmt.Errorf("lock order: %w", err)
	}

	return nil
}

func (r *Repository) CheckIdempotencyKey(ctx context.Context, orderID uuid.UUID, op domain.Operation) (bool, error) {
	const query = `SELECT 1 FROM idempotency_keys WHERE order_id = $1 AND operation = $2`

	var dummy int
	err := r.txGetter.DefaultTrOrDB(ctx, r.pool).QueryRow(ctx, query, orderID, op).Scan(&dummy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("check idempotency key: %w", err)
	}

	return true, nil
}

func (r *Repository) SaveIdempotencyKey(ctx context.Context, orderID uuid.UUID, op domain.Operation) error {
	const query = `
        INSERT INTO idempotency_keys (order_id, operation) 
        VALUES ($1, $2)
        ON CONFLICT (order_id, operation) DO NOTHING
    `

	if _, err := r.txGetter.DefaultTrOrDB(ctx, r.pool).Exec(ctx, query, orderID, op); err != nil {
		return fmt.Errorf("save idempotency key: %w", err)
	}

	return nil
}
