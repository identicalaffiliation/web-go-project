package feedbacks

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	trm "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/identicalaffiliation/web-go-project/backend/catalog/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	poolRO   *pgxpool.Pool
	pool     *pgxpool.Pool
	txGetter *trm.CtxGetter
}

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		poolRO:   replica,
		pool:     master,
		txGetter: trm.DefaultCtxGetter,
	}
}

// CreateFeedback - отзыв только на неудалённый товар: FK удалённый (deleted_at) товар пропускает,
// поэтому отсутствующий или удалённый товар - ErrNotFound.
func (r *Repository) CreateFeedback(ctx context.Context, f *domain.Feedback) (*domain.Feedback, error) {
	const query = `INSERT INTO feedbacks (id, product_id, user_id, score, description)
		SELECT $1::uuid, $2::uuid, $3::uuid, $4::smallint, $5::text
		WHERE EXISTS (SELECT 1 FROM catalog WHERE product_id = $2::uuid AND deleted_at IS NULL)
		RETURNING *`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.pool)

	var m feedbackModel
	err := conn.QueryRow(ctx, query, f.ID, f.ProductID, f.UserID, f.Score, f.Description).Scan(
		&m.ID,
		&m.ProductID,
		&m.UserID,
		&m.Score,
		&m.Description,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("error CreateFeedback: %w", err)
	}

	return m.toDomain(), nil
}

func (r *Repository) GetMyCreatedFeedbacks(ctx context.Context, userId uuid.UUID) ([]*domain.Feedback, error) {
	const query = `SELECT * FROM feedbacks WHERE user_id = $1`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.poolRO)

	rows, err := conn.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("error GetMyFeedbacks: %w", err)
	}

	t, err := pgx.CollectRows(rows, pgx.RowToStructByName[feedbackModel])
	if err != nil {
		return nil, fmt.Errorf("error GetMyFeedbacks: %w", err)
	}

	feedbacks := make([]*domain.Feedback, 0, len(t))
	for _, v := range t {
		feedbacks = append(feedbacks, v.toDomain())
	}

	return feedbacks, nil
}

// GetProductFeedbacks - отзывы товара, курсор - id последнего отзыва (uuid v7, упорядочен по времени).
func (r *Repository) GetProductFeedbacks(ctx context.Context, productID uuid.UUID, cursor *uuid.UUID, limit int64) ([]*domain.Feedback, error) {
	const query = `
		SELECT * FROM feedbacks
		WHERE product_id = $1
			AND id < COALESCE($2::uuid, 'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid) -- uuid max filter
		ORDER BY id DESC
		LIMIT $3::bigint
	`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.poolRO)

	rows, err := conn.Query(ctx, query, productID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("error GetProductFeedbacks: %w", err)
	}

	t, err := pgx.CollectRows(rows, pgx.RowToStructByName[feedbackModel])
	if err != nil {
		return nil, fmt.Errorf("error GetProductFeedbacks: %w", err)
	}

	feedbacks := make([]*domain.Feedback, 0, len(t))
	for _, v := range t {
		feedbacks = append(feedbacks, v.toDomain())
	}

	return feedbacks, nil
}

func (r *Repository) DeleteFeedback(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM feedbacks WHERE id = $1`

	conn := r.txGetter.DefaultTrOrDB(ctx, r.pool)

	tag, err := conn.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error DeleteFeedback: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
