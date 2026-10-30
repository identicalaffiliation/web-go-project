package catalog

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/adapters/postgres/utils"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/jackc/pgx/v5"
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

// feedbacksStats - агрегаты отзывов по товару. LATERAL отдаёт ровно одну строку на товар,
// поэтому LIMIT в пагинации продолжает считать товары, а не отзывы.
const feedbacksStats = `
	LEFT JOIN LATERAL (
		SELECT
			COALESCE(round(avg(f.score), 2), 0)::float8 AS rating,
			count(*) AS feedbacks_count
		FROM feedbacks f
		WHERE f.product_id = c.product_id
	) fs ON true
`

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		pool:   master,
		poolRO: replica,
		getter: trm.DefaultCtxGetter,
	}
}

func (r *Repository) GetItemsByCursorPagination(ctx context.Context, cursor *string, limit int64) ([]*domain.Product, error) {
	const query = `
		SELECT c.product_id, c.title, c.description, c.price, c.image_key, c.created_at, c.updated_at,
			fs.rating, fs.feedbacks_count
		FROM catalog c` + feedbacksStats + `
		WHERE c.product_id < COALESCE($1::uuid, 'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid) -- uuid max filter
		ORDER BY c.product_id DESC
		LIMIT $2::bigint
	`

	var c any
	if cursor != nil && *cursor != "" {
		id, err := uuid.Parse(*cursor)
		if err != nil {
			return nil, ErrInvalidData
		}

		c = id
	}

	conn := r.getter.DefaultTrOrDB(ctx, r.poolRO)
	rows, err := conn.Query(ctx, query, c, limit)
	if err != nil {
		return nil, fmt.Errorf("error GetItemsByCursorPagination: %w", err)
	}

	t, err := pgx.CollectRows(rows, pgx.RowToStructByName[productModel])
	if err != nil {
		return nil, fmt.Errorf("error GetItemsByCursorPagination: %w", err)
	}

	models := make([]*domain.Product, 0, len(t))
	for _, v := range t {
		models = append(models, v.toDomain())
	}

	return models, nil
}

func (r *Repository) GetItemByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	const query = `
		SELECT c.product_id, c.title, c.description, c.price, c.image_key, c.created_at, c.updated_at,
			fs.rating, fs.feedbacks_count
		FROM catalog c` + feedbacksStats + `
		WHERE c.product_id = $1
	`

	var m productModel
	conn := r.getter.DefaultTrOrDB(ctx, r.poolRO)
	err := conn.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.Title,
		&m.Description,
		&m.Price,
		&m.ImageKey,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.Rating,
		&m.FeedbacksCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("error GetItemByID: %w", err)
	}

	return m.toDomain(), nil
}

func (r *Repository) Insert(ctx context.Context, product *domain.Product) (*domain.Product, error) {
	const query = `
		INSERT INTO catalog (product_id, title, description, price)
		VALUES ($1, $2, $3, $4) RETURNING
		product_id, title, description, price, image_key, created_at, updated_at
	`

	var (
		desc *string
	)

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

func (r *Repository) UpdateImageKey(ctx context.Context, key string, productID uuid.UUID) (*domain.Product, error) {
	// агрегаты нужны и здесь: результат кладётся в кэш карточки
	const query = `
		WITH c AS (
			UPDATE catalog SET image_key = $1, updated_at = now() WHERE product_id = $2
			RETURNING product_id, title, description, price, image_key, created_at, updated_at
		)
		SELECT c.product_id, c.title, c.description, c.price, c.image_key, c.created_at, c.updated_at,
			fs.rating, fs.feedbacks_count
		FROM c` + feedbacksStats

	conn := r.getter.DefaultTrOrDB(ctx, r.pool)

	var m productModel
	err := conn.QueryRow(ctx, query, key, productID).Scan(
		&m.ID,
		&m.Title,
		&m.Description,
		&m.Price,
		&m.ImageKey,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.Rating,
		&m.FeedbacksCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("error UpdateImageKey: %w", err)
	}

	return m.toDomain(), nil
}
