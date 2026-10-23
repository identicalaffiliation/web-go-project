package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	checkCode  = "23514"
	uniqueCode = "23505"
)

var (
	ErrInvalidData   = errors.New("invalid data")
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

func validateConstraints(err error) error {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		if strings.EqualFold(pgErr.Code, checkCode) {
			return ErrInvalidData
		}

		if strings.EqualFold(pgErr.Code, uniqueCode) {
			return ErrAlreadyExists
		}
	}

	return fmt.Errorf("create order: %w", err)
}
