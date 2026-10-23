package postgres

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/orders/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool     *pgxpool.Pool
	poolRO   *pgxpool.Pool
	txGetter *trm.CtxGetter
}

func NewRepository(pool *pgxpool.Pool, poolRO *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, poolRO: poolRO, txGetter: trm.DefaultCtxGetter}
}

// CreateOrder создает новый заказ со статусом по умолчанию: 'created'.
// Использует ключ идемпотентности от клиента, который гарантирует, что
// при повтором запросе заказ создаться только один. Запись идет только в master базу.
func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	const query = `INSERT INTO orders 
    	(id, user_id, product_id, operation_id, quantity, total_price)
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING *`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.pool)
	var model orderModel
	err := conn.QueryRow(ctx, query,
		order.ID,
		order.UserID,
		order.ProductID,
		order.OperationID,
		order.Quantity,
		order.TotalPrice,
	).Scan(
		&model.ID,
		&model.UserID,
		&model.ProductID,
		&model.OperationID,
		&model.Quantity,
		&model.TotalPrice,
		&model.Status,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, validateConstraints(err)
	}

	return model.toDomain(), nil
}

// GetOrdersByUserID метод получения последних 30 заказов пользователя.
// Скорость работы при увеличении размера базы данных гарантирует btree индекс на user_id + created_at DESC.
// При SELECT запросе, вне транзакции, читаем с реплики, чтобы не нагружать master базу.
// Репликация между нодами при помощи WAL.
func (r *Repository) GetOrdersByUserID(ctx context.Context, id uuid.UUID) ([]*domain.Order, error) {
	const query = `SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT 30`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.poolRO)
	rows, err := conn.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("select orders: %w", err)
	}

	t, err := pgx.CollectRows[orderModel](rows, pgx.RowToStructByName)
	if err != nil {
		return nil, fmt.Errorf("collect orders: %w", err)
	}

	models := make([]*domain.Order, 0, len(t))
	for i := range t {
		models = append(models, t[i].toDomain())
	}

	return models, nil
}

// UpdateStatusByOrderID TODO
func (r *Repository) UpdateStatusByOrderID(ctx context.Context, id uuid.UUID, newStatus domain.Status) error {
	return nil
}
