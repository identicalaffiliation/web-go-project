package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
)

const uniqueViolation = "23505"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	id, err := encodeUUID(string(user.ID))
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO auth_users (id, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4::user_role, $5)
	`

	_, err = r.pool.Exec(ctx, query, id, string(user.Email), user.PasswordHash, string(user.Role), user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	const query = `
		SELECT id, email, password_hash, role, created_at
		FROM auth_users
		WHERE email = $1
	`
	return r.scanUser(r.pool.QueryRow(ctx, query, string(email)))
}

func (r *UserRepository) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	pgID, err := encodeUUID(string(id))
	if err != nil {
		return domain.User{}, err
	}

	const query = `
		SELECT id, email, password_hash, role, created_at
		FROM auth_users
		WHERE id = $1
	`
	return r.scanUser(r.pool.QueryRow(ctx, query, pgID))
}

func (r *UserRepository) scanUser(row pgx.Row) (domain.User, error) {
	var (
		id           pgtype.UUID
		email        string
		passwordHash string
		role         string
		createdAt    time.Time
	)

	err := row.Scan(&id, &email, &passwordHash, &role, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}

	return domain.User{
		ID:           domain.UserID(decodeUUID(id)),
		Email:        domain.Email(email),
		PasswordHash: passwordHash,
		Role:         domain.Role(role),
		CreatedAt:    createdAt,
	}, nil
}

func encodeUUID(s string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse uuid %q: %w", s, err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func decodeUUID(v pgtype.UUID) string {
	return uuid.UUID(v.Bytes).String()
}
