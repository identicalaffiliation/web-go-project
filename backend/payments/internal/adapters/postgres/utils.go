package postgres

import (
	"errors"
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
)

// validateConstraints - маппит нарушения constraint в ошибки адаптера, остальные ошибки возвращает как есть.
func validateConstraints(err error) error {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		if strings.EqualFold(pgErr.Code, checkCode) {
			return ErrInvalidData
		}

		if strings.EqualFold(pgErr.Code, uniqueCode) {
			return ErrAlreadyExists
		}
	}

	return err
}
