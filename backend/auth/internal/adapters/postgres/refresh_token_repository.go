package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token domain.RefreshToken) error {
	return r.insert(ctx, r.pool, token)
}

func (r *RefreshTokenRepository) insert(ctx context.Context, q execer, token domain.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := q.Exec(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.RevokedAt, token.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var token domain.RefreshToken

	err := r.pool.QueryRow(ctx, query, hash).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt, &token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, ports.ErrRefreshTokenNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("scan refresh token: %w", err)
	}

	return token, nil
}

func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldID uuid.UUID, next domain.RefreshToken, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`, now, oldID); err != nil {
		return fmt.Errorf("revoke old refresh token: %w", err)
	}

	if err := r.insert(ctx, tx, next); err != nil {
		return fmt.Errorf("insert rotated refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, now time.Time) error {
	cmd, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`, now, id)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ports.ErrRefreshTokenNotFound
	}
	return nil
}
