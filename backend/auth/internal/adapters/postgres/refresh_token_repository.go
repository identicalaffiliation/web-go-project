package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
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
	id, err := encodeUUID(token.ID)
	if err != nil {
		return err
	}
	userID, err := encodeUUID(string(token.UserID))
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = q.Exec(ctx, query, id, userID, token.TokenHash, token.ExpiresAt, token.RevokedAt, token.CreatedAt)
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

	var (
		id        pgtype.UUID
		userID    pgtype.UUID
		tokenHash string
		expiresAt time.Time
		revokedAt *time.Time
		createdAt time.Time
	)

	err := r.pool.QueryRow(ctx, query, hash).Scan(&id, &userID, &tokenHash, &expiresAt, &revokedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("scan refresh token: %w", err)
	}

	return domain.RefreshToken{
		ID:        decodeUUID(id),
		UserID:    domain.UserID(decodeUUID(userID)),
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
		CreatedAt: createdAt,
	}, nil
}

func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldID string, next domain.RefreshToken, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // если Commit уже прошёл — это no-op

	oldPgID, err := encodeUUID(oldID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`, now, oldPgID); err != nil {
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

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string, now time.Time) error {
	pgID, err := encodeUUID(id)
	if err != nil {
		return err
	}

	cmd, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`, now, pgID)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrRefreshTokenNotFound
	}
	return nil
}
