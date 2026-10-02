package catalog

import (
	"context"
	"errors"
	"fmt"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/utils"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrInvalidData = errors.New("invalid data")
)

type Repository struct {
	pool   *pgxpool.Pool
	poolRO *pgxpool.Pool
	getter *trm.CtxGetter
}

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		pool:   master,
		poolRO: replica,
		getter: trm.DefaultCtxGetter,
	}
}

func (r *Repository) Insert(ctx context.Context, product *domain.Product) (*domain.Product, error) {
	const query = `
		INSERT INTO catalog (product_id, title, description, price, image_key)
		VALUES ($1, $2, $3, $4, $5) RETURNING
		product_id, title, description, price, image_key, created_at, updated_at
	`

	var (
		key  *string
		desc *string
	)
	if product.ImageKey != "" {
		key = &product.ImageKey
	}

	if product.Description != "" {
		desc = &product.Description
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.pool)
	var m productModel
	err := conn.QueryRow(
		ctx,
		query,
		product.ID,
		product.Title,
		desc,
		product.Price,
		key,
	).Scan(
		&m.ID,
		&m.Title,
		&m.Description,
		&m.Price,
		&m.ImageKey,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if utils.IsCheckConstraint(err) {
			return nil, ErrInvalidData
		}

		return nil, fmt.Errorf("error CatalogInsert: %w", err)
	}

	return m.toDomain(), nil
}
