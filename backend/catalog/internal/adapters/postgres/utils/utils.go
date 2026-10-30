package utils

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	checkCode   = "23514"
	foreignCode = "23503"
)

func IsCheckConstraint(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return strings.EqualFold(checkCode, pgErr.Code)
	}

	return false
}

func IsForeignConstraint(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return strings.EqualFold(foreignCode, pgErr.Code)
	}

	return false
}
