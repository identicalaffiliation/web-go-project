package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

const uniqueViolation = "23505"

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	return insertUser(ctx, r.pool, user)
}

func (r *UserRepository) CreateWithEvent(ctx context.Context, user domain.User, event domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// После успешного Commit Rollback вернёт ErrTxClosed и ничего не сделает;
	// при любой ошибке ниже откатит обе вставки разом.
	defer func() { _ = tx.Rollback(ctx) }()

	if err := insertUser(ctx, tx, user); err != nil {
		return err
	}
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func insertUser(ctx context.Context, q execer, user domain.User) error {
	const query = `
		INSERT INTO auth_users (id, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4::user_role, $5)
	`

	_, err := q.Exec(ctx, query, user.ID, user.Email, user.PasswordHash, user.Role, user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
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
	return r.scanUser(r.pool.QueryRow(ctx, query, email))
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const query = `
		SELECT id, email, password_hash, role, created_at
		FROM auth_users
		WHERE id = $1
	`
	return r.scanUser(r.pool.QueryRow(ctx, query, id))
}

// UpdateRole ставит роль одним UPDATE без предварительного чтения. Повтор с той
// же ролью затрагивает ту же одну строку, поэтому не ошибка; 0 строк значит, что
// пользователя нет. Вариант «AND role <> $2» не отличил бы «роль уже такая» от
// «пользователя нет» без второго запроса.
func (r *UserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role domain.Role) error {
	const query = `UPDATE auth_users SET role = $2::user_role WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, role)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User

	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ports.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}

	return user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
